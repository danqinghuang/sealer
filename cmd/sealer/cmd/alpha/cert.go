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

package alpha

import (
	"context"
	"fmt"
	"strings"

	"github.com/sealerio/sealer/utils/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"

	"github.com/sealerio/sealer/pkg/clusterfile"
)

var altNames []string

var longCertCmdDescription = `This command will add the new domain or IP address in cert to update cluster API server.

sealer has some default domain and IP in the cert process builtin: localhost,outbound IP address and some DNS domain which is strongly related to the apiserver CertSANs configured by kubeadm.yml.

You need to restart your API server manually after using sealer alpha cert. Then, you can using cmd "openssl x509 -noout -text -in apiserver.crt" to check the cert details.
`

var exampleForCertCmd = `
The following command will generate new api server cert and key for all control-plane certificates:

sealer alpha cert --alt-names 39.105.169.253,sealer.cool
`

func NewCertCmd() *cobra.Command {
	certCmd := &cobra.Command{
		Use:     "cert",
		Short:   "Update Kubernetes API server's cert",
		Args:    cobra.NoArgs,
		Long:    longCertCmdDescription,
		Example: exampleForCertCmd,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(altNames) == 0 {
				return fmt.Errorf("IP address or DNS domain needed for cert Subject Alternative Names")
			}

			cluster, err := clusterfile.GetDefaultCluster()
			if err != nil {
				return fmt.Errorf("failed to get default cluster: %v", err)
			}

			certUpdateCmd := fmt.Sprintf("seautil cert update --alt-names %s", strings.Join(altNames, ","))
			// send new cert to all master.
			ips := cluster.GetMasterIPList()
			eg, _ := errgroup.WithContext(context.Background())
			for _, ip := range ips {
				node := ip
				eg.Go(func() error {
					sshClient, err := ssh.NewStdoutSSHClient(node, cluster)
					if err != nil {
						return fmt.Errorf("failed to new ssh client: %v", err)
					}
					return sshClient.CmdAsync(node, certUpdateCmd)
				})
			}
			return eg.Wait()
		},
	}

	certCmd.Flags().StringSliceVar(&altNames, "alt-names", []string{}, "add DNS domain or IP in certs, if it is already in the cert subject alternative names list, nothing will be changed")

	return certCmd
}
