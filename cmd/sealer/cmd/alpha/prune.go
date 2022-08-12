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
	"fmt"
	"os"
	"path/filepath"

	"github.com/sealerio/sealer/pkg/prune"

	"github.com/spf13/cobra"
)

var exampleForPruneCmd = `The following command will prune sealer data directory, such as: image db, image layer, build tmp.

sealer alpha prune
`

var longPruneCmdDescription = ``

// NewPruneCmd returns the sealer filesystem prune Cobra command
func NewPruneCmd() *cobra.Command {
	pruneCmd := &cobra.Command{
		Use:     "prune",
		Short:   "Prune sealer data dir",
		Long:    longPruneCmdDescription,
		Args:    cobra.NoArgs,
		Example: exampleForPruneCmd,
		RunE:    pruneAction,
	}

	return pruneCmd
}

func pruneAction(cmd *cobra.Command, args []string) error {
	buildTmp := prune.NewBuildPrune()
	ima, err := prune.NewImagePrune()
	if err != nil {
		return err
	}
	layer, err := prune.NewLayerPrune()
	if err != nil {
		return err
	}
	for _, pruneService := range []prune.Pruner{ima, layer, buildTmp} {
		trashList, err := pruneService.Select()
		if err != nil {
			return err
		}

		fmt.Printf("%s ... \n", pruneService.GetSelectorMessage())
		for _, trash := range trashList {
			if err := os.RemoveAll(trash); err != nil {
				return err
			}
			fmt.Printf("%s deleted\n", filepath.Base(trash))
		}
	}

	return nil
}
