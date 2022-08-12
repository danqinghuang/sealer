// Copyright © 2021 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudfilesystem

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/sealerio/sealer/pkg/registry"

	"github.com/sealerio/sealer/common"
	"github.com/sealerio/sealer/pkg/env"
	v2 "github.com/sealerio/sealer/types/api/v2"
	utilsnet "github.com/sealerio/sealer/utils/net"
	"github.com/sealerio/sealer/utils/platform"
	"github.com/sealerio/sealer/utils/ssh"

	"golang.org/x/sync/errgroup"
)

const (
	RemoteChmod = "cd %s  && chmod +x scripts/* && cd scripts && bash init.sh /var/lib/docker %s %s"
)

type overlayFileSystem struct {
}

func (o *overlayFileSystem) MountRootfs(cluster *v2.Cluster, hosts []net.IP, initFlag bool) error {
	clusterRootfsDir := common.DefaultTheClusterRootfsDir(cluster.Name)
	//scp roofs to all Masters and Nodes,then do init.sh
	if err := mountRootfs(hosts, clusterRootfsDir, cluster, initFlag); err != nil {
		return fmt.Errorf("failed to mount rootfs(%s): %v", clusterRootfsDir, err)
	}
	return nil
}

func (o *overlayFileSystem) UnMountRootfs(cluster *v2.Cluster, hosts []net.IP) error {
	//do clean.sh,then remove all Masters and Nodes roofs
	if err := unmountRootfs(hosts, cluster); err != nil {
		return err
	}
	return nil
}

func mountRootfs(ipList []net.IP, target string, cluster *v2.Cluster, initFlag bool) error {
	clusterPlatform, err := ssh.GetClusterPlatform(cluster)
	if err != nil {
		return err
	}
	mountEntry := struct {
		*sync.RWMutex
		mountDirs map[string]bool
	}{&sync.RWMutex{}, make(map[string]bool)}
	config := registry.GetConfig(platform.DefaultMountClusterImageDir(cluster.Name), cluster.GetMaster0IP())
	eg, _ := errgroup.WithContext(context.Background())
	for _, IP := range ipList {
		ip := IP
		eg.Go(func() error {
			src := platform.GetMountClusterImagePlatformDir(cluster.Name, clusterPlatform[ip.String()])
			initCmd := fmt.Sprintf(RemoteChmod, target, config.Domain, config.Port)
			mountEntry.Lock()
			if !mountEntry.mountDirs[src] {
				mountEntry.mountDirs[src] = true
			}
			mountEntry.Unlock()
			sshClient, err := ssh.GetHostSSHClient(ip, cluster)
			if err != nil {
				return fmt.Errorf("failed to get ssh client of host(%s): %v", ip, err)
			}
			err = copyFiles(sshClient, ip, src, target)
			if err != nil {
				return fmt.Errorf("failed to copy rootfs: %v", err)
			}
			if initFlag {
				err = sshClient.CmdAsync(ip, env.NewEnvProcessor(cluster).WrapperShell(ip, initCmd))
				if err != nil {
					return fmt.Errorf("failed to exec init.sh: %v", err)
				}
			}
			return err
		})
	}
	if err = eg.Wait(); err != nil {
		return err
	}
	// if config.ip is not in mountRootfs ipList, mean copy registry dir is not required, like scale up node
	if utilsnet.NotInIPList(config.IP, ipList) {
		return nil
	}
	return copyRegistry(config.IP, cluster, mountEntry.mountDirs, target)
}

func unmountRootfs(ipList []net.IP, cluster *v2.Cluster) error {
	var (
		clusterRootfsDir = common.DefaultTheClusterRootfsDir(cluster.Name)
		cleanFile        = fmt.Sprintf(common.DefaultClusterClearBashFile, cluster.Name)
		unmount          = fmt.Sprintf("(! mountpoint -q %[1]s || umount -lf %[1]s)", clusterRootfsDir)
		execClean        = fmt.Sprintf("if [ -f \"%[1]s\" ];then chmod +x %[1]s && /bin/bash -c %[1]s;fi", cleanFile)
		rmRootfs         = fmt.Sprintf("rm -rf %s", clusterRootfsDir)
		envProcessor     = env.NewEnvProcessor(cluster)
		cmd              = strings.Join([]string{execClean, unmount, rmRootfs}, " && ")
	)

	eg, _ := errgroup.WithContext(context.Background())
	for _, IP := range ipList {
		ip := IP
		eg.Go(func() error {
			SSH, err := ssh.GetHostSSHClient(ip, cluster)
			if err != nil {
				return err
			}

			return SSH.CmdAsync(ip, envProcessor.WrapperShell(ip, cmd))
		})
	}
	return eg.Wait()
}

func NewOverlayFileSystem() (Interface, error) {
	return &overlayFileSystem{}, nil
}
