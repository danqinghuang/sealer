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

package cmd

import (
	"github.com/sealerio/sealer/pkg/runtime/kubernetes"
	"github.com/spf13/cobra"

	"github.com/sealerio/sealer/apply"
)

var clusterFile string

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "apply a Kubernetes cluster via specified Clusterfile",
	Long: `apply command is used to apply a Kubernetes cluster via specified Clusterfile.
If the Clusterfile is applied first time, Kubernetes cluster will be created. Otherwise, sealer
will apply the diff change of current Clusterfile and the original one.`,
	Example: `sealer apply -f Clusterfile`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		applier, err := apply.NewApplierFromFile(clusterFile)
		if err != nil {
			return err
		}
		return applier.Apply()
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVarP(&clusterFile, "Clusterfile", "f", "Clusterfile", "Clusterfile path to apply a Kubernetes cluster")
	applyCmd.Flags().BoolVar(&kubernetes.ForceDelete, "force", false, "force to delete the specified cluster if set true")
}
