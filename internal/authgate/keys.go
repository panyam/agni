package authgate

import (
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/panyam/oneauth/core"
	oagae "github.com/panyam/oneauth/stores/gae"
	oagorm "github.com/panyam/oneauth/stores/gorm"
	"gorm.io/gorm"
)

// Keys is the store of the server's tokens: oneauth's API key store, with the listing and the expiry
// sweep of its APIKeyAdmin that List and Prune use (agni issue 1733, panyam/oneauth#549). Store keeps
// them as files under a --store folder, SQLStore as rows of a sqlite or postgres database, and
// GAEStore as Datastore entities.
type Keys interface {
	core.APIKeyStore
	core.APIKeyAdmin
}

// SQLStore is the store of the server's tokens in db, creating oneauth's api_keys table if absent.
func SQLStore(db *gorm.DB) (Keys, error) {
	if err := db.AutoMigrate(&oagorm.APIKeyModel{}); err != nil {
		return nil, fmt.Errorf("tokens: %w", err)
	}
	return oagorm.NewAPIKeyStore(db), nil
}

// GAEStore is the store of the server's tokens in Datastore, in namespace.
func GAEStore(client *datastore.Client, namespace string) Keys {
	return oagae.NewAPIKeyStore(client, namespace)
}
