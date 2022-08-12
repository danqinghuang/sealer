// Copyright © 2022 Alibaba Group Holding Ltd.
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

package kubernetes

import (
	"fmt"
	"net"
	"path"
	"path/filepath"
	"strings"

	"github.com/sealerio/sealer/pkg/clustercert/cert"

	"github.com/sealerio/sealer/common"
	"github.com/sealerio/sealer/utils/exec"
	osi "github.com/sealerio/sealer/utils/os"
	"github.com/sealerio/sealer/utils/ssh"

	"github.com/pkg/errors"
)

func GetKubectlAndKubeconfig(ssh ssh.Interface, host net.IP, rootfs string) error {
	// fetch the cluster kubeconfig, and add /etc/hosts "EIP apiserver.cluster.local" so we can get the current cluster status later
	err := ssh.Fetch(host, path.Join(common.DefaultKubeConfigDir(), "config"), common.KubeAdminConf)
	if err != nil {
		return errors.Wrap(err, "failed to copy kubeconfig")
	}
	_, err = exec.RunSimpleCmd(fmt.Sprintf("cat /etc/hosts |grep '%s %s' || echo '%s %s' >> /etc/hosts",
		host, common.APIServerDomain, host, common.APIServerDomain))
	if err != nil {
		return errors.Wrap(err, "failed to add master IP to etc hosts")
	}

	if !osi.IsFileExist(common.KubectlPath) {
		err = osi.RecursionCopy(filepath.Join(rootfs, "bin/kubectl"), common.KubectlPath)
		if err != nil {
			return err
		}
		err = exec.Cmd("chmod", "+x", common.KubectlPath)
		if err != nil {
			return errors.Wrap(err, "failed to chmod a+x kubectl")
		}
	}
	return nil
}

func GenerateRegistryCert(registryCertPath string, baseName string) error {
	regCertConfig := cert.CertificateDescriptor{
		CommonName:   baseName,
		DNSNames:     []string{baseName},
		Organization: []string{common.ExecBinaryFileName},
		Year:         100,
	}
	if baseName != SeaHub {
		regCertConfig.DNSNames = append(regCertConfig.DNSNames, SeaHub)
	}

	caGenerator := cert.NewAuthorityCertificateGenerator(regCertConfig)
	caCert, caKey, err := caGenerator.Generate()
	if err != nil {
		return fmt.Errorf("unable to generate %s cert: %v", baseName, err)
	}

	// write cert file to disk
	err = cert.NewCertificateFileManger(registryCertPath, baseName).Write(caCert, caKey)
	if err != nil {
		return fmt.Errorf("unable to save %s cert: %v", baseName, err)
	}

	return nil
}

func getEtcdEndpointsWithHTTPSPrefix(masters []net.IP) string {
	var tmpSlice []string
	for _, ip := range masters {
		tmpSlice = append(tmpSlice, fmt.Sprintf("https://%s:2379", ip))
	}
	return strings.Join(tmpSlice, ",")
}
