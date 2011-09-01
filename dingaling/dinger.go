package dingaling

import (
	"appengine"
	"appengine/channel"
	"appengine/datastore"
	"encoding/base64"
	"big"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"regexp"
	"os"
)

type Dinger struct {
	Name string
}

type Client struct {
	Id    string
	Token string // The channel token
}

const (
	DINGER_KEY_KIND = "Dinger"
	CLIENT_KEY_KIND = "DingerClient"
)

// A dinger's ID is just a number hex-encoded and so must match this regexp
var validDingerId = regexp.MustCompile("^[0-9A-Fa-f]+$")

// Parse a dinger ID into a datastore key
func DingerIdToKey(idStr string) (*datastore.Key, os.Error) {
	// Sanity check against a regexp. We do this because Sscanf will accept things like "-4" which are not
	// valid dinger ids.
	if !validDingerId.MatchString(idStr) {
		return nil, os.NewError(fmt.Sprintf("Invalid dinger id: %v", idStr))
	}

	// Parse the id as an int64
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return nil, os.NewError(fmt.Sprintf("Error parsing dinger id: %v", err))
	}

	// Create and return the key
	return datastore.NewKey(DINGER_KEY_KIND, "", int64(id), nil), nil
}

// Return the (URL safe) string ID associated with the dinger key
func KeyToDingerId(key *datastore.Key) string {
	return fmt.Sprint(key.IntID())
}

// Make a new Dinger with the specified name and try to insert it in the data store. Return the Dinger, the
// key associated with the new dinger. If insertion/creation failed, return an error explaining why.
func MakeDinger(c appengine.Context, name string) (*Dinger, *datastore.Key, os.Error) {
	// Make a dinger with the specified name
	dinger := &Dinger{Name: name}

	// Attempt to insert the dinger in the datastore
	key, err := datastore.Put(c, datastore.NewIncompleteKey(DINGER_KEY_KIND), dinger)
	if err != nil {
		return nil, nil, err
	}
	return dinger, key, nil
}

// Convenience function to get a dinger from the data store.
func GetDinger(c appengine.Context, key *datastore.Key) (*Dinger, os.Error) {
	var dinger Dinger
	if err := datastore.Get(c, key, &dinger); err != nil {
		return nil, err
	}
	return &dinger, nil
}

// Make a client of the specified dinger and return the id and token associated with the new client.
func MakeClient(c appengine.Context, dingerKey *datastore.Key) (*Client, os.Error) {
	// Create a key for this client
	key := datastore.NewKey(CLIENT_KEY_KIND, "", 0, dingerKey)

	// Generate some id for the client. It'll be the SHA1 sum of the dinger id and some random number.
	hashWriter := sha1.New()
	fmt.Fprint(hashWriter, KeyToDingerId(dingerKey))

	// Append a random integer
	randId, err := rand.Int(rand.Reader, big.NewInt(0x7fffffffffffffff))
	if err != nil {
		return nil, err
	}
	fmt.Fprint(hashWriter, randId)

	// Base64 encode the client id
	id := base64.URLEncoding.EncodeToString(hashWriter.Sum())

	// Create a channel for this client.
	tok, err := channel.Create(c, id)
	if err != nil {
		return nil, err
	}

	// Create the client record and store it
	client := &Client{Id: id, Token: tok}
	key, err = datastore.Put(c, key, client)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func PostDing(c appengine.Context, key *datastore.Key, message string) os.Error {
        // If no message was passed, use a default
        if len(message) == 0 {
                message = "Ding-A-Ling!"
        }

        // Return all clients associated with this dinger
        q := datastore.NewQuery(CLIENT_KEY_KIND).Ancestor(key)

        // Iterate over all clients
        for i := q.Run(c); ; {
                var client Client
                cKey, err := i.Next(&client)
                if err == datastore.Done {
                        break;
                } else if err != nil {
                        return err
                }

                // Send the message to this client
                err = channel.Send(c, client.Id, message)
                if err != nil {
                        // Error sending message, remove this client from the data store. Log any errors but
                        // don't report them to the user
                        err = datastore.Delete(c, cKey)
                        if err != nil {
                                c.Errorf("Error deleting client: %v", err)
                        }
                }
        }

        return nil
}
