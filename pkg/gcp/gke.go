/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package gcp

import (
	"encoding/base64"
	"fmt"

	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/kubefirst/runtime/pkg/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// ListContainerClusters
func (conf *GCPConfiguration) ListContainerClusters() (*containerpb.ListClustersResponse, error) {
	client, err := container.NewClusterManagerClient(conf.Context)
	if err != nil {
		return nil, fmt.Errorf("could not create google container client: %s", err)
	}

	clusters, err := client.ListClusters(conf.Context, &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/-", conf.Project),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing container clusters: %s", err)
	}

	return clusters, nil
}

// GetContainerCluster
func (conf *GCPConfiguration) GetContainerCluster(clusterName string) (*containerpb.Cluster, error) {
	client, err := container.NewClusterManagerClient(conf.Context)
	if err != nil {
		return nil, fmt.Errorf("could not create google container client: %s", err)
	}

	cluster, err := client.GetCluster(conf.Context, &containerpb.GetClusterRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", conf.Project, conf.Region, clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting container cluster: %s", err)
	}

	return cluster, nil
}

// GetContainerClusterAuth
func (conf *GCPConfiguration) GetContainerClusterAuth(clusterName string) (*k8s.KubernetesClient, error) {
	client, err := container.NewClusterManagerClient(conf.Context)
	if err != nil {
		return nil, fmt.Errorf("could not create google container client: %s", err)
	}

	cluster, err := client.GetCluster(conf.Context, &containerpb.GetClusterRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", conf.Project, conf.Region, clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting container cluster: %s", err)
	}

	config := api.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   map[string]*api.Cluster{},
		AuthInfos:  map[string]*api.AuthInfo{},
		Contexts:   map[string]*api.Context{},
	}

	name := fmt.Sprintf("gke_%s_%s_%s", conf.Project, cluster.Location, cluster.Name)
	cert, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate cluster=%s cert=%s: %w", name, cluster.MasterAuth.ClusterCaCertificate, err)
	}
	config.Clusters[name] = &api.Cluster{
		CertificateAuthorityData: cert,
		Server:                   "https://" + cluster.Endpoint,
	}
	config.Contexts[name] = &api.Context{
		Cluster:  name,
		AuthInfo: name,
	}
	config.AuthInfos[name] = &api.AuthInfo{
		AuthProvider: &api.AuthProviderConfig{
			Name: "gcp",
			Config: map[string]string{
				"scopes": "https://www.googleapis.com/auth/cloud-platform",
			},
		},
	}

	var kubeConfig *rest.Config
	var clientset *kubernetes.Clientset

	for clusterName := range config.Clusters {
		kubeConfig, err = clientcmd.NewNonInteractiveClientConfig(config, clusterName, &clientcmd.ConfigOverrides{CurrentContext: clusterName}, nil).ClientConfig()
		if err != nil {
			fmt.Printf("error building kubernetes config: %s", err)
		}

		clientset, err = kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			fmt.Printf("error buildling kubernetes clientset: %s", err)
		}
	}

	return &k8s.KubernetesClient{
		Clientset:  clientset,
		RestConfig: kubeConfig,
	}, nil
}
