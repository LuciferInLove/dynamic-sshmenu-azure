package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2020-10-01/resources"
	"golang.org/x/sync/errgroup"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

type azureSession struct {
	SubscriptionID string
	Authorizer     autorest.Authorizer
}

type resourceGroup struct {
	Number   string
	Name     string
	Location string
}

type virtualMachine struct {
	Number string
	IP     string
	Name   string
}

func parseAuthFile(authFilePath string) (*map[string]interface{}, error) {
	data, err := ioutil.ReadFile(authFilePath)

	if err != nil {
		return nil, fmt.Errorf("Can't authenticate.\n%w", err)
	}

	contents := make(map[string]interface{})
	err = json.Unmarshal(data, &contents)

	if err != nil {
		return nil, fmt.Errorf("Can't get authentication info.\n%w", err)
	}

	return &contents, err
}

func newSessionFromFile() (*azureSession, error) {
	authorizer, err := auth.NewAuthorizerFromFile(azure.PublicCloud.ResourceManagerEndpoint)

	if err != nil {
		return nil, fmt.Errorf("Can't authenticate.\n%w", err)
	}

	authInfo, err := parseAuthFile(os.Getenv("AZURE_AUTH_LOCATION"))

	if err != nil {
		return nil, fmt.Errorf("Can't get authentication info.\n%w", err)
	}

	session := azureSession{
		SubscriptionID: (*authInfo)["subscriptionId"].(string),
		Authorizer:     authorizer,
	}

	return &session, nil
}

func getResourceGroups(session *azureSession, location string) ([]string, error) {
	var (
		i              int      = 1
		resourceGroups []string = make([]string, 0)
	)

	groupsClient := resources.NewGroupsClient(session.SubscriptionID)
	groupsClient.Authorizer = session.Authorizer

	for list, err := groupsClient.ListComplete(context.Background(), "", nil); list.NotDone(); err = list.Next() {
		if err != nil {
			return resourceGroups, fmt.Errorf("Can't traverse Azure Resource Groups list.\n%w", err)
		}
		if location != "" && location != *list.Value().Location {
			continue
		}
		currentGroup := resourceGroup{
			Number:   strconv.Itoa(i),
			Name:     *list.Value().Name,
			Location: *list.Value().Location,
		}
		currentGroupString, err := json.Marshal(currentGroup)
		if err != nil {
			return resourceGroups, err
		}

		resourceGroups = append(resourceGroups, string(currentGroupString))
		i++
	}
	return resourceGroups, nil
}

func getVM(session *azureSession, resourceGroup string, tags string, publicIP bool) ([]string, error) {
	var (
		filter          string = "resourceType eq 'Microsoft.Compute/virtualMachines'"
		vmValues        []compute.VirtualMachine
		vmValuesChannel chan compute.VirtualMachine = make(chan compute.VirtualMachine)
		vmDefinitions   []string                    = make([]string, 0)
		ctx             context.Context             = context.Background()
	)

	// Handle goroutine errors
	errs, gctx := errgroup.WithContext(ctx)

	vmClient := compute.NewVirtualMachinesClient(session.SubscriptionID)
	vmClient.Authorizer = session.Authorizer

	resourcesClient := resources.NewClient(session.SubscriptionID)
	resourcesClient.Authorizer = session.Authorizer

out:
	for vmResource, err := resourcesClient.ListByResourceGroupComplete(ctx, resourceGroup, filter, "", nil); vmResource.NotDone(); err = vmResource.Next() {
		if err != nil {
			return vmDefinitions, err
		}

		vmValue := vmResource.Value()

		// Filter VM by tags if defined
		if tags != "" {
			vmTags := vmValue.Tags
			if len(vmTags) == 0 {
				continue
			}

			tagsList := strings.Split(tags, ";")
			for _, tag := range tagsList {
				keyValue := strings.Split(tag, ":")
				if len(keyValue) != 2 {
					return vmDefinitions, fmt.Errorf("WrongTagDefinition")
				}

				if value, ok := vmTags[keyValue[0]]; ok {
					if keyValue[1] != *value {
						continue out
					}
				} else {
					continue out
				}
			}
		}

		// Append VM to vmValuesChannel if VM is running
		errs.Go(func() error {
			vm, err := vmClient.Get(gctx, resourceGroup, *vmValue.Name, compute.InstanceViewTypesInstanceView)
			if err != nil {
				return err
			}

			statusCode := *(*vm.InstanceView.Statuses)[1].Code
			if statusCode == "PowerState/running" {
				select {
				case vmValuesChannel <- vm:
					return nil
				case <-gctx.Done():
					return gctx.Err()
				}
			}

			return nil
		})
	}

	go func() {
		errs.Wait()
		close(vmValuesChannel)
	}()

	for vmValue := range vmValuesChannel {
		vmValues = append(vmValues, vmValue)
	}

	vmDefinitions = make([]string, len(vmValues))

	for index, vmValue := range vmValues {
		index := index
		vmValue := vmValue

		// Get VM ip and append VM definition to vmDefinitions
		errs.Go(func() error {
			vmDefinition, err := getVMDefinition(session, resourceGroup, vmValue, index+1, publicIP)
			if err != nil {
				return err
			}

			vmDefinitions[index] = vmDefinition
			return nil
		})
	}

	return vmDefinitions, errs.Wait()
}

func getVMDefinition(session *azureSession, resourceGroup string, vmValue compute.VirtualMachine, i int, publicIP bool) (string, error) {
	var (
		vmDefinition string
		ipAddress    string          = "(no public ip)"
		ctx          context.Context = context.Background()
	)

	networkClient := network.NewInterfacesClient(session.SubscriptionID)
	networkClient.Authorizer = session.Authorizer

	networkInterfaceSlice := strings.Split(*(*vmValue.NetworkProfile.NetworkInterfaces)[0].ID, "/")
	networkInterface, err := networkClient.Get(ctx, resourceGroup, networkInterfaceSlice[len(networkInterfaceSlice)-1], "")
	if err != nil {
		return vmDefinition, err
	}

	// Check if publicIP is set
	if publicIP {
		publicIPAddress := (*networkInterface.IPConfigurations)[0].PublicIPAddress
		if publicIPAddress != nil {
			publicIPClient := network.NewPublicIPAddressesClient(session.SubscriptionID)
			publicIPClient.Authorizer = session.Authorizer
			publicIPAddressInfo, err := publicIPClient.Get(ctx, resourceGroup, *publicIPAddress.Name, "")
			if err != nil {
				return vmDefinition, err
			}
			ipAddress = *publicIPAddressInfo.IPAddress
		}
	} else {
		ipAddress = *(*networkInterface.IPConfigurations)[0].PrivateIPAddress
	}

	// Generate current VM definition
	currentVM := virtualMachine{
		Number: strconv.Itoa(i),
		Name:   *vmValue.Name,
		IP:     ipAddress,
	}
	currentVMString, err := json.Marshal(currentVM)
	if err != nil {
		return vmDefinition, err
	}

	return string(currentVMString), nil
}
