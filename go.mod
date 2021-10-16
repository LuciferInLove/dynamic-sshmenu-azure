module dynamic-sshmenu-azure

go 1.16

require (
	github.com/Azure/azure-sdk-for-go v58.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.21
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.8
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/manifoldco/promptui v0.8.0
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
)

replace github.com/manifoldco/promptui v0.8.0 => github.com/LuciferInLove/promptui v0.7.1-0.20201003113208-3398ab7c53db
