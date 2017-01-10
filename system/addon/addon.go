package addon

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/ponzu-cms/ponzu/system/db"
	"github.com/ponzu-cms/ponzu/system/item"
)

var (
	// Types is a record of addons, like content types, of addon_reverse_dns:interface{}
	Types = make(map[string]func() interface{})
)

const (
	// StatusEnabled defines string status for Addon enabled state
	StatusEnabled = "enabled"
	// StatusDisabled defines string status for Addon disabled state
	StatusDisabled = "disabled"
)

// Meta contains the basic information about the addon
type Meta struct {
	PonzuAddonName       string `json:"addon_name"`
	PonzuAddonAuthor     string `json:"addon_author"`
	PonzuAddonAuthorURL  string `json:"addon_author_url"`
	PonzuAddonVersion    string `json:"addon_version"`
	PonzuAddonReverseDNS string `json:"addon_reverse_dns"`
	PonzuAddonStatus     string `json:"addon_status"`
}

// Addon contains information about a provided addon to the system
type Addon struct {
	item.Item
	Meta
}

// Register sets up the system to use the Addon by:
// 1. Adding Meta to the Addon struct
// 2. Saving it to the __addons bucket in DB with id/key = addon_reverse_dns
// 3. Checking that the Addon parent type was added to Types (likely via its init())
func Register(meta Meta, addon Addon) error {
	a := Addon{Meta: meta}

	// get or create the reverse DNS identifier
	if a.PonzuAddonReverseDNS == "" {
		revDNS, err := reverseDNS(meta)
		if err != nil {
			return err
		}

		a.PonzuAddonReverseDNS = revDNS
	}

	if _, ok := Types[a.PonzuAddonReverseDNS]; !ok {
		panic(`Addon "` + a.PonzuAddonName + `" has no record in the addons.Types map`)
	}

	// check if addon is already registered in db as addon_reverse_dns
	if db.AddonExists(a.PonzuAddonReverseDNS) {
		return nil
	}

	// convert a.Item into usable data, Item{} => []byte(json) => map[string]interface{}
	kv := make(map[string]interface{})

	data, err := json.Marshal(a.Item)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &kv)
	if err != nil {
		return err
	}

	// save new addon to db
	vals := make(url.Values)
	for k, v := range kv {
		vals.Set(k, v.(string))
	}

	vals.Set("addon_name", a.PonzuAddonName)
	vals.Set("addon_author", a.PonzuAddonAuthor)
	vals.Set("addon_author_url", a.PonzuAddonAuthorURL)
	vals.Set("addon_version", a.PonzuAddonVersion)
	vals.Set("addon_reverse_dns", a.PonzuAddonReverseDNS)
	vals.Set("addon_status", StatusDisabled)

	// db.SetAddon is like SetContent, but rather than the key being an int64 ID,
	// we need it to be a string based on the addon_reverse_dns
	err = db.SetAddon(vals)
	if err != nil {
		return err
	}

	return nil
}

// Deregister removes an addon from the system. `key` is the addon_reverse_dns
func Deregister(key string) error {
	err := db.DeleteAddon(key)
	if err != nil {
		return err
	}

	delete(Types, key)
	return nil
}

// Enable sets the addon status to `enabled`. `key` is the addon_reverse_dns
func Enable(key string) error {
	err := setStatus(key, StatusEnabled)
	if err != nil {
		return err
	}

	return nil
}

// Disable sets the addon status to `disabled`. `key` is the addon_reverse_dns
func Disable(key string) error {
	err := setStatus(key, StatusDisabled)
	if err != nil {
		return err
	}

	return nil
}

func setStatus(key, status string) error {
	a, err := db.Addon(key)
	if err != nil {
		return err
	}

	a.Set("addon_status", status)

	err = db.SetAddon(a)
	if err != nil {
		return err
	}

	return nil
}

func reverseDNS(meta Meta) (string, error) {
	u, err := url.Parse(meta.PonzuAddonAuthorURL)
	if err != nil {
		return "", nil
	}

	if u.Host == "" {
		return "", fmt.Errorf(`Error parsing Addon Author URL: %s. Ensure URL is formatted as "scheme://hostname/path?query" (path & query optional)`, meta.PonzuAddonAuthorURL)
	}

	name := strings.Replace(meta.PonzuAddonName, " ", "", -1)

	// reverse the host name parts, split on '.', ex. bosssauce.it => it.bosssauce
	parts := strings.Split(u.Host, ".")
	strap := make([]string, len(parts), len(parts))
	for i := len(parts) - 1; i >= 0; i-- {
		strap = append(strap, parts[i])
	}

	return strings.Join(append(strap, name), "."), nil
}
