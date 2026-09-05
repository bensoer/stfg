package scrape

import (
	"fmt"
	"stfg/internal"
	"strings"
	"time"

	"go.uber.org/zap"
)

func Reconcile(scrapeClient ScrapeClient, storageClient StorageClient, options ScrapeReconcilerOptions) error {
	log := zap.S() //.With("source", "ScrapeReconciler")

	// iterate over storageClient RetailGroups and remove whats invalid
	rtgs, err := storageClient.GetAllRetailGroups()
	if err != nil {
		return err
	}
	for _, rtg := range rtgs {
		retailGroupIsValid, err := RetailGroupIsValid(rtg)
		if err != nil {
			return err
		}
		if !retailGroupIsValid {

			log.Infof("Retail Group %s - %s Is Invalid. Removing", rtg.Merchant, rtg.Name)
			log.Infof("Removing All Retail Group Items For Invalid Retail Group %s - %s", rtg.Merchant, rtg.Name)
			retailGroupItems, err := storageClient.GetRetailGroupItems(rtg.ID)
			if err != nil {
				return err
			}
			for _, retailGroupItem := range retailGroupItems {
				storageClient.RemoveRetailGroupItem(retailGroupItem)
			}
			err = storageClient.RemoveRetailGroup(rtg)
			if err != nil {
				return err
			}

		}
	}
	// All expired RetailGroups are now gone, so now we can add new stuff

	// iterate over retail groups
	rtgs, err = scrapeClient.GetRetailGroups(options.PostalCode)
	if err != nil {
		return err
	}

	for _, retailGroup := range rtgs {

		// Validation check that we aren't being given invalid retail groups from the scrapClient
		retailGroupIsValid, err := RetailGroupIsValid(retailGroup)
		if err != nil {
			return err
		}
		if !retailGroupIsValid {
			continue
		}

		// Filter only RetailGroups that we want to see
		found := false
		for _, whitelist := range options.RetailGroupWhiteList {
			fullName := fmt.Sprintf("%s %s", retailGroup.Merchant, retailGroup.Name)
			if strings.Contains(fullName, whitelist) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// NOTE: We assume that no RetailItems can have different validity timelines then the parent RetailGroup
		// All RetailItems are valid within the RetailGroups start and end dates.

		// At this point we can gaurantee the retail group does not exist OR the retail group exists and is still valid

		// If it does not exist, add it!
		if !storageClient.HasRetailGroup(retailGroup) {
			log.Infof("Adding Retail Group \"%s - %s\" And Its Items", retailGroup.Merchant, retailGroup.Name)
			err := storageClient.AddRetailGroup(retailGroup)
			if err != nil {
				return err
			}

			rtgis, err := scrapeClient.GetRetailGroupItems(retailGroup.ID)
			if err != nil {
				return err
			}

			for _, retailGroupItem := range rtgis {
				hasRetailGroupItem, err := storageClient.HasRetailGroupItem(retailGroupItem)
				if err != nil {
					return err
				}
				if !hasRetailGroupItem {
					err := storageClient.AddRetailGroupItem(retailGroupItem)
					if err != nil {
						return err
					}
				}
			}
		}

		// If it does already exist. Then we do nothing. Assuming nothing has changed thus

	}

	return nil
}

func RetailGroupIsValid(retailGroup RetailGroup) (bool, error) {

	validFrom, err := internal.ParseDate(retailGroup.ValidFrom)
	if err != nil {
		return false, err
	}

	validTo, err := internal.ParseDate(retailGroup.ValidTo)
	if err != nil {
		return false, err
	}

	now := time.Now()
	if now.After(validFrom) && now.Before(validTo) {
		return true, nil
	} else {
		return false, nil
	}
}
