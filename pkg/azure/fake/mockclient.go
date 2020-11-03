/*
 * Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/marketplaceordering/mgmt/marketplaceordering"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	api "github.com/gardener/machine-controller-manager-provider-azure/pkg/azure/apis"
	"github.com/gardener/machine-controller-manager-provider-azure/pkg/azure/apis/validation"
	"github.com/gardener/machine-controller-manager-provider-azure/pkg/client"
	"github.com/gardener/machine-controller-manager-provider-azure/pkg/spi"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
)

//PluginSPIImpl is the mock implementation of PluginSPIImpl
type PluginSPIImpl struct {
	SPI spi.SessionProviderInterface
}

// NewFakeAzureDriver returns an empty AzureDriver object
func NewFakeAzureDriver(spi spi.SessionProviderInterface) *PluginSPIImpl {
	return &PluginSPIImpl{
		SPI: spi,
	}
}

//Setup creates a compute service instance using the mock
func (ms *PluginSPIImpl) Setup(secret *corev1.Secret) (*client.AzureDriverClients, error) {

	var (
		subscriptionID = strings.TrimSpace(string(secret.Data[v1alpha1.AzureSubscriptionID]))
		tenantID       = strings.TrimSpace(string(secret.Data[v1alpha1.AzureTenantID]))
		clientID       = strings.TrimSpace(string(secret.Data[v1alpha1.AzureClientID]))
		clientSecret   = strings.TrimSpace(string(secret.Data[v1alpha1.AzureClientSecret]))
		env            = azure.PublicCloud
	)

	newClients, err := NewClients(subscriptionID, tenantID, clientID, clientSecret, env)
	if err != nil {
		return nil, err
	}

	return newClients, nil
}

// NewClients returns the authenticated Azure clients
func NewClients(subscriptionID, tenantID, clientID, clientSecret string, env azure.Environment) (*client.AzureDriverClients, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, env.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	subnetClient := network.NewSubnetsClient(subscriptionID)
	subnetClient.Authorizer = authorizer

	interfacesClient := network.NewInterfacesClient(subscriptionID)
	interfacesClient.Authorizer = authorizer

	vmClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmClient.Authorizer = authorizer

	vmImagesClient := compute.NewVirtualMachineImagesClient(subscriptionID)
	vmImagesClient.Authorizer = authorizer

	diskClient := compute.NewDisksClient(subscriptionID)
	diskClient.Authorizer = authorizer

	deploymentsClient := resources.NewDeploymentsClient(subscriptionID)
	deploymentsClient.Authorizer = authorizer

	marketplaceClient := marketplaceordering.NewMarketplaceAgreementsClient(subscriptionID)
	marketplaceClient.Authorizer = authorizer

	return &client.AzureDriverClients{Subnet: subnetClient, Nic: interfacesClient, VM: vmClient, Disk: diskClient, Deployments: deploymentsClient, Images: vmImagesClient, Marketplace: marketplaceClient}, nil
}

// decodeProviderSpecAndSecret unmarshals the raw providerspec into api.AzureProviderSpec structure
func decodeProviderSpecAndSecret(machineClass *v1alpha1.MachineClass, secret *corev1.Secret) (*api.AzureProviderSpec, error) {
	var providerSpec *api.AzureProviderSpec

	// Extract providerSpec
	err := json.Unmarshal(machineClass.ProviderSpec.Raw, &providerSpec)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	//Validate the Spec and Secrets
	ValidationErr := validation.ValidateAzureSpecNSecret(providerSpec, secret)
	if ValidationErr != nil {
		err = fmt.Errorf("Error while validating ProviderSpec %v", ValidationErr)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return providerSpec, nil
}

// CreateMachine ...
func (ms *PluginSPIImpl) CreateMachine(ctx context.Context, req *driver.CreateMachineRequest) (*driver.CreateMachineResponse, error) {

	var providerSpec *api.AzureProviderSpec

	providerSpec, err := decodeProviderSpecAndSecret(req.MachineClass, req.Secret)

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	providerID := fmt.Sprintf("azure:///%s/%s", providerSpec.Location, req.Machine.Name)
	return &driver.CreateMachineResponse{ProviderID: providerID, NodeName: req.Machine.Name}, nil
}

// DeleteMachine ...
func (ms *PluginSPIImpl) DeleteMachine(ctx context.Context, req *driver.DeleteMachineRequest) (*driver.DeleteMachineResponse, error) {

	return &driver.DeleteMachineResponse{}, nil
}