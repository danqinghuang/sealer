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

package apply

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/sealerio/sealer/apply/driver"
	"github.com/sealerio/sealer/common"
	v1 "github.com/sealerio/sealer/types/api/v1"
	v2 "github.com/sealerio/sealer/types/api/v2"
	"github.com/sealerio/sealer/utils/hash"
	utilsnet "github.com/sealerio/sealer/utils/net"
	strUtils "github.com/sealerio/sealer/utils/strings"
	"github.com/sealerio/sealer/utils/yaml"
)

// NewScaleApplierFromArgs will filter ip list from command parameters.
func NewScaleApplierFromArgs(clusterfile string, scaleArgs *Args, flag string) (driver.Interface, error) {
	if scaleArgs.Nodes == "" && scaleArgs.Masters == "" {
		return nil, fmt.Errorf("master and node cannot both be empty")
	}

	// validate input masters IP info
	if len(scaleArgs.Masters) != 0 {
		if err := validateIPStr(scaleArgs.Masters); err != nil {
			return nil, fmt.Errorf("failed to validate input scale masters ip: %v", err)
		}
	}

	// validate input nodes IP info
	if len(scaleArgs.Nodes) != 0 {
		if err := validateIPStr(scaleArgs.Nodes); err != nil {
			return nil, fmt.Errorf("failed to validate input scale nodes ip: %v", err)
		}
	}

	cluster := &v2.Cluster{}
	if err := yaml.UnmarshalFile(clusterfile, cluster); err != nil {
		return nil, err
	}

	var err error
	switch flag {
	case common.JoinSubCmd:
		err = Join(cluster, scaleArgs)
	case common.DeleteSubCmd:
		err = Delete(cluster, scaleArgs)
	}
	if err != nil {
		return nil, err
	}

	applier, err := NewDefaultApplier(cluster)
	if err != nil {
		return nil, err
	}
	return applier, nil
}

func Join(cluster *v2.Cluster, scaleArgs *Args) error {
	return joinBaremetalNodes(cluster, scaleArgs)
}

func joinBaremetalNodes(cluster *v2.Cluster, scaleArgs *Args) error {
	var err error
	// merge custom Env to the existed cluster
	cluster.Spec.Env = append(cluster.Spec.Env, scaleArgs.CustomEnv...)

	scaleArgs.Masters, err = utilsnet.AssemblyIPList(scaleArgs.Masters)
	if err != nil {
		return err
	}

	scaleArgs.Nodes, err = utilsnet.AssemblyIPList(scaleArgs.Nodes)
	if err != nil {
		return err
	}

	if (!utilsnet.IsIPList(scaleArgs.Nodes) && scaleArgs.Nodes != "") || (!utilsnet.IsIPList(scaleArgs.Masters) && scaleArgs.Masters != "") {
		return fmt.Errorf("parameter error: current mode should submit iplist")
	}

	// if scaleArgs`s ssh auth credential is different from local cluster,will add it to each host.
	// if not use local cluster ssh auth credential.
	var changedSSH *v1.SSH

	passwd := cluster.Spec.SSH.Passwd
	if cluster.Spec.SSH.Encrypted {
		passwd, err = hash.AesDecrypt([]byte(cluster.Spec.SSH.Passwd))
		if err != nil {
			return err
		}
	}

	if scaleArgs.Password != "" && scaleArgs.Password != passwd {
		// Encrypt password here to avoid merge failed.
		passwd, err = hash.AesEncrypt([]byte(scaleArgs.Password))
		if err != nil {
			return err
		}
		changedSSH = &v1.SSH{
			Encrypted: true,
			User:      scaleArgs.User,
			Passwd:    passwd,
			Pk:        scaleArgs.Pk,
			PkPasswd:  scaleArgs.PkPassword,
			Port:      strconv.Itoa(int(scaleArgs.Port)),
		}
	}

	//add joined masters
	if scaleArgs.Masters != "" {
		masterIPs := cluster.GetMasterIPList()
		addedMasterIPStr := removeDuplicate(strings.Split(scaleArgs.Masters, ","))
		addedMasterIP := utilsnet.IPStrsToIPs(addedMasterIPStr)

		for _, ip := range addedMasterIP {
			// if ip already taken by master will return join duplicated ip error
			if !utilsnet.NotInIPList(ip, masterIPs) {
				return fmt.Errorf("failed to scale master for duplicated ip: %s", ip)
			}
		}

		host := v2.Host{
			IPS:   addedMasterIP,
			Roles: []string{common.MASTER},
		}

		if changedSSH != nil {
			host.SSH = *changedSSH
		}

		cluster.Spec.Hosts = append(cluster.Spec.Hosts, host)
	}

	//add joined nodes
	if scaleArgs.Nodes != "" {
		nodeIPs := cluster.GetNodeIPList()
		addedNodeIPStrs := removeDuplicate(strings.Split(scaleArgs.Nodes, ","))
		addedNodeIP := utilsnet.IPStrsToIPs(addedNodeIPStrs)

		for _, ip := range addedNodeIP {
			// if ip already taken by node will return join duplicated ip error
			if !utilsnet.NotInIPList(ip, nodeIPs) {
				return fmt.Errorf("failed to scale node for duplicated ip: %s", ip)
			}
		}

		host := v2.Host{
			IPS:   addedNodeIP,
			Roles: []string{common.NODE},
		}

		if changedSSH != nil {
			host.SSH = *changedSSH
		}

		cluster.Spec.Hosts = append(cluster.Spec.Hosts, host)
	}
	return nil
}

func removeDuplicate(ipList []string) []string {
	return strUtils.RemoveDuplicate(strUtils.NewComparator(ipList, []string{""}).GetSrcSubtraction())
}

func Delete(cluster *v2.Cluster, scaleArgs *Args) error {
	return deleteBaremetalNodes(cluster, scaleArgs)
}

func deleteBaremetalNodes(cluster *v2.Cluster, scaleArgs *Args) error {
	var err error
	// adding custom Env params for delete option here to support executing users clean scripts via env.
	cluster.Spec.Env = append(cluster.Spec.Env, scaleArgs.CustomEnv...)

	scaleArgs.Masters, err = utilsnet.AssemblyIPList(scaleArgs.Masters)
	if err != nil {
		return err
	}

	scaleArgs.Nodes, err = utilsnet.AssemblyIPList(scaleArgs.Nodes)
	if err != nil {
		return err
	}

	if (!utilsnet.IsIPList(scaleArgs.Nodes) && scaleArgs.Nodes != "") || (!utilsnet.IsIPList(scaleArgs.Masters) && scaleArgs.Masters != "") {
		return fmt.Errorf("parameter error: current mode should submit iplist")
	}

	//master0 machine cannot be deleted
	scaleMasterIPs := utilsnet.IPStrsToIPs(strings.Split(scaleArgs.Masters, ","))
	if !utilsnet.NotInIPList(cluster.GetMaster0IP(), scaleMasterIPs) {
		return fmt.Errorf("master0 machine(%s) cannot be deleted", cluster.GetMaster0IP())
	}

	if scaleArgs.Masters != "" && utilsnet.IsIPList(scaleArgs.Masters) {
		for i := range cluster.Spec.Hosts {
			if !strUtils.NotIn(common.MASTER, cluster.Spec.Hosts[i].Roles) {
				masterIPs := utilsnet.IPStrsToIPs(strings.Split(scaleArgs.Masters, ","))
				cluster.Spec.Hosts[i].IPS = returnFilteredIPList(cluster.Spec.Hosts[i].IPS, masterIPs)
			}
		}
	}
	if scaleArgs.Nodes != "" && utilsnet.IsIPList(scaleArgs.Nodes) {
		for i := range cluster.Spec.Hosts {
			if !strUtils.NotIn(common.NODE, cluster.Spec.Hosts[i].Roles) {
				nodeIPs := utilsnet.IPStrsToIPs(strings.Split(scaleArgs.Nodes, ","))
				cluster.Spec.Hosts[i].IPS = returnFilteredIPList(cluster.Spec.Hosts[i].IPS, nodeIPs)
			}
		}
	}
	return nil
}

func returnFilteredIPList(clusterIPList []net.IP, toBeDeletedIPList []net.IP) (res []net.IP) {
	for _, ip := range clusterIPList {
		if utilsnet.NotInIPList(ip, toBeDeletedIPList) {
			res = append(res, ip)
		}
	}
	return
}
