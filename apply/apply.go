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
	"os"
	"path/filepath"

	"github.com/sealerio/sealer/apply/driver"
	"github.com/sealerio/sealer/common"
	"github.com/sealerio/sealer/pkg/clusterfile"
	"github.com/sealerio/sealer/pkg/filesystem"
	"github.com/sealerio/sealer/pkg/image"
	"github.com/sealerio/sealer/pkg/image/store"
	v2 "github.com/sealerio/sealer/types/api/v2"
)

type Args struct {
	ClusterName string

	// Masters and Nodes only support:
	// IP list format: ip1,ip2,ip3
	// IP range format: x.x.x.x-x.x.x.y
	Masters string
	Nodes   string

	User       string
	Password   string
	Port       uint16
	Pk         string
	PkPassword string
	PodCidr    string
	SvcCidr    string
	Provider   string
	CustomEnv  []string
	CMDArgs    []string
}

func NewApplierFromFile(path string) (driver.Interface, error) {
	if !filepath.IsAbs(path) {
		pa, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(pa, path)
	}
	Clusterfile, err := clusterfile.NewClusterFile(path)
	if err != nil {
		return nil, err
	}
	imgSvc, err := image.NewImageService()
	if err != nil {
		return nil, err
	}

	mounter, err := filesystem.NewClusterImageMounter()
	if err != nil {
		return nil, err
	}

	is, err := store.NewDefaultImageStore()
	if err != nil {
		return nil, err
	}
	cluster := Clusterfile.GetCluster()
	if cluster.Name == "" {
		return nil, fmt.Errorf("cluster name cannot be empty, make sure %s file is correct", path)
	}
	if cluster.GetAnnotationsByKey(common.ClusterfileName) == "" {
		cluster.SetAnnotations(common.ClusterfileName, path)
	}
	return &driver.Applier{
		ClusterDesired:      &cluster,
		ClusterFile:         Clusterfile,
		ImageManager:        imgSvc,
		ClusterImageMounter: mounter,
		ImageStore:          is,
	}, nil
}

// NewApplier news an applier.
// In NewApplier, we guarantee that no raw data could be passed in.
// And all data has to be validated and processed in the pre-process layer.
func NewApplier(cluster *v2.Cluster) (driver.Interface, error) {
	return NewDefaultApplier(cluster)
}

func NewDefaultApplier(cluster *v2.Cluster) (driver.Interface, error) {
	if cluster.Name == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}
	imgSvc, err := image.NewImageService()
	if err != nil {
		return nil, err
	}

	mounter, err := filesystem.NewClusterImageMounter()
	if err != nil {
		return nil, err
	}

	is, err := store.NewDefaultImageStore()
	if err != nil {
		return nil, err
	}

	return &driver.Applier{
		ClusterDesired:      cluster,
		ImageManager:        imgSvc,
		ClusterImageMounter: mounter,
		ImageStore:          is,
	}, nil
}
