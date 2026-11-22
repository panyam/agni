package authgate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/panyam/oneauth/core"
	oafs "github.com/panyam/oneauth/stores/fs"
	oagae "github.com/panyam/oneauth/stores/gae"
	oagorm "github.com/panyam/oneauth/stores/gorm"
	"gorm.io/gorm"
)

// Keys is the store of the server's tokens: oneauth's API key store, plus the listing and the expiry
// sweep that List and Prune need and oneauth's interface lacks. Store keeps them as files under a
// --store folder, SQLStore as rows of a sqlite or postgres database, and GAEStore as Datastore
// entities (agni issue 1733).
type Keys interface {
	core.APIKeyStore
	// all returns every token, hashes included.
	all(ctx context.Context) ([]*core.APIKey, error)
	// deleteExpired deletes the tokens whose expiry is at or before now, and returns how many.
	deleteExpired(ctx context.Context, now time.Time) (int, error)
}

// fsKeys reads oneauth's token folder, `<store>/tokens/api_keys/<id>.json`, for the two calls
// oneauth's FSAPIKeyStore has no method for.
type fsKeys struct{ *oafs.FSAPIKeyStore }

func (k fsKeys) dir() string { return filepath.Join(k.StoragePath, "api_keys") }

// all reads the folder. A token deleted after the folder is read, as the expiry sweep does while the
// server runs, is left out rather than failing the listing (agni issue 1796).
func (k fsKeys) all(ctx context.Context) ([]*core.APIKey, error) {
	entries, err := os.ReadDir(k.dir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*core.APIKey
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		resp, err := k.GetAPIKeyByID(ctx, &core.GetAPIKeyByIDRequest{KeyID: id})
		if errors.Is(err, core.ErrAPIKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, resp.APIKey)
	}
	return out, nil
}

func (k fsKeys) deleteExpired(ctx context.Context, now time.Time) (int, error) {
	keys, err := k.all(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, key := range keys {
		if key.ExpiresAt == nil || key.ExpiresAt.After(now) {
			continue
		}
		if err := os.Remove(filepath.Join(k.dir(), key.KeyID+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return n, err
		}
		n++
	}
	return n, nil
}

// sqlKeys queries oneauth's api_keys table through its exported model for the two calls oneauth's
// GORM APIKeyStore has no method for.
type sqlKeys struct {
	*oagorm.APIKeyStore
	db *gorm.DB
}

// SQLStore is the store of the server's tokens in db, creating oneauth's api_keys table if absent.
func SQLStore(db *gorm.DB) (Keys, error) {
	if err := db.AutoMigrate(&oagorm.APIKeyModel{}); err != nil {
		return nil, fmt.Errorf("tokens: %w", err)
	}
	return sqlKeys{oagorm.NewAPIKeyStore(db), db}, nil
}

func (k sqlKeys) all(ctx context.Context) ([]*core.APIKey, error) {
	var rows []oagorm.APIKeyModel
	if err := k.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*core.APIKey, len(rows))
	for i := range rows {
		out[i] = rows[i].ToAPIKey()
	}
	return out, nil
}

func (k sqlKeys) deleteExpired(ctx context.Context, now time.Time) (int, error) {
	res := k.db.WithContext(ctx).Where("expires_at IS NOT NULL AND expires_at <= ?", now).Delete(&oagorm.APIKeyModel{})
	return int(res.RowsAffected), res.Error
}

// gaeKeys lists oneauth's APIKey entities in Datastore for the two calls oneauth's Datastore
// APIKeyStore has no method for. It reads each key back through the store, so oneauth still owns
// the entity's fields.
type gaeKeys struct {
	*oagae.APIKeyStore
	client    *datastore.Client
	namespace string
}

// GAEStore is the store of the server's tokens in Datastore, in namespace.
func GAEStore(client *datastore.Client, namespace string) Keys {
	return gaeKeys{oagae.NewAPIKeyStore(client, namespace), client, namespace}
}

func (k gaeKeys) ids(ctx context.Context) ([]*datastore.Key, error) {
	return k.client.GetAll(ctx, datastore.NewQuery(oagae.KindAPIKey).Namespace(k.namespace).KeysOnly(), nil)
}

func (k gaeKeys) all(ctx context.Context) ([]*core.APIKey, error) {
	ids, err := k.ids(ctx)
	if err != nil {
		return nil, err
	}
	var out []*core.APIKey
	for _, id := range ids {
		resp, err := k.GetAPIKeyByID(ctx, &core.GetAPIKeyByIDRequest{KeyID: id.Name})
		if errors.Is(err, core.ErrAPIKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, resp.APIKey)
	}
	return out, nil
}

func (k gaeKeys) deleteExpired(ctx context.Context, now time.Time) (int, error) {
	keys, err := k.all(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, key := range keys {
		if key.ExpiresAt == nil || key.ExpiresAt.After(now) {
			continue
		}
		dk := datastore.NameKey(oagae.KindAPIKey, key.KeyID, nil)
		dk.Namespace = k.namespace
		if err := k.client.Delete(ctx, dk); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
