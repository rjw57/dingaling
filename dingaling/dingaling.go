// Dingaling is a simple AppEngine application which allows ad hoc
// notifications to be set up.
package dingaling

import (
	"appengine"
	"appengine/datastore"
	"fmt"
	"http"
	"io/ioutil"
	"os"
	"strings"
	"template"
)

// Some components of our URLs.
const (
	ROOT          = "/"
	DINGER_PREFIX = "/d/"
)

var (
        rootTemplate, dingerTemplate *template.Template // initialised in init()
)

// Set up the http handler functions for dingaling.
func init() {
	http.HandleFunc(ROOT, wrapHandler(rootHandler))
	http.HandleFunc(DINGER_PREFIX, wrapHandler(handleDinger))

        initTemplate := func(fn string) (*template.Template, os.Error) {
                t := template.New(nil)
                t.SetDelims("{{{", "}}}")
                if err := t.ParseFile(fn); err != nil {
                        return nil, os.NewError("Cannot parse dinger.html template: " + err.String())
                }
                return t, nil;
        }

        var err os.Error

        if rootTemplate, err = initTemplate("root.html"); err != nil {
                panic(err.String());
        }

        if dingerTemplate, err = initTemplate("dinger.html"); err != nil {
                panic(err.String());
        }
}

// Handle the root URL
func rootHandler(c appengine.Context, w http.ResponseWriter, r *http.Request) os.Error {
	return rootTemplate.Execute(w, DINGER_PREFIX)
}

// A handler function for URLs starting with DINGER_PREFIX. Create a dinger if none is specified, otherwise
// return a JSON object describing the dinger and a channel for it
func handleDinger(c appengine.Context, w http.ResponseWriter, r *http.Request) os.Error {
	// Check URL's path is as expected
	if !strings.HasPrefix(r.URL.Path, DINGER_PREFIX) {
		return os.NewError(fmt.Sprintf(
			"handleDinger was passed a URL without expected prefix of '%v': %v", DINGER_PREFIX,
			r.URL.Path))
	}

	// Extract sub-path
	dingerKeyStr := r.URL.Path[len(DINGER_PREFIX):]

	// Sub-path should be of the form <ID>[/request]
	requestStr := ""
	if slashIdx := strings.Index(dingerKeyStr, "/"); slashIdx != -1 {
		requestStr = dingerKeyStr[slashIdx+1:]
		dingerKeyStr = dingerKeyStr[:slashIdx]
	}

	// No key id? We want a new dinger then.
	if len(dingerKeyStr) == 0 {
		return handleNewDinger(c, w, r)
	}

	// Parse the dinger key
	key, err := DingerIdToKey(dingerKeyStr)
	if err != nil {
		return err
	}

	// Is this a post request?
	if r.Method == "POST" {
		if requestStr != "" {
			return os.NewError(fmt.Sprintf("Malformed URL for dinger POST request: %v", r.URL.Path))
		}
		return handleDingerPost(c, w, r, key)
	}

	// Handle the existing dinger
	switch requestStr {
		case "":
			return dingerTemplate.Execute(w, dingerUrl(r, key))
		case "info":
			return handleDingerInfo(c, w, r, key)
		case "connect":
			return handleDingerConnect(c, w, r, key)
		default:
			return os.NewError(fmt.Sprintf("Malformed URL for dinger request: %v", r.URL.Path))
	}

	panic("Not reached")
}

// Handle requests for information on a dinger.
func handleDingerInfo(c appengine.Context, w http.ResponseWriter, r *http.Request, key *datastore.Key) os.Error {
	// Retrieve the dinger record associated with this key
	dinger, err := GetDinger(c, key)
	if err != nil {
		return err
	}

	return jsonResponse(w, dinger)
}

// Handle requests for posting to a dinger
func handleDingerPost (c appengine.Context, w http.ResponseWriter, r *http.Request, key *datastore.Key) os.Error {
	// Read the request body
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Convert the body to a string
	body := string(bodyBytes)

	// Post it to all clients
	return PostDing(c, key, body)
}

// Handle attempts by new clients to connect to this dinger.
func handleDingerConnect(c appengine.Context, w http.ResponseWriter, r *http.Request, key *datastore.Key) os.Error {
	// Create a new client for this dinger
	client, err := MakeClient(c, key)
	if err != nil {
		return err
	}

	// Return the client structure
	return jsonResponse(w, client)
}

func dingerUrl(r *http.Request, key *datastore.Key) *http.URL {
	// Return a URL to the client which can be used to access this dinger. The URL is based on the request
	// URL. See the http.URL documentation for why these particular fields are set.
	dingerUrl := r.URL
	dingerUrl.RawQuery = ""
	dingerUrl.Fragment = ""
	dingerUrl.Path = fmt.Sprintf("%v%v", DINGER_PREFIX, KeyToDingerId(key))

	return dingerUrl
}

// A handler function for when we should create a new dinger.
func handleNewDinger(c appengine.Context, w http.ResponseWriter, r *http.Request) os.Error {
	// Was a name requested?
	name := r.FormValue("name")
	if len(name) == 0 {
		name = "Untitled dinger"
	}

	// Create a dinger with the specified name
	_, key, err := MakeDinger(c, name)
	if err != nil {
		return err
	}

	type Response struct {
		Name string
		URL  string
	}

	// Create and return the response to the client.
	return jsonResponse(w, Response{name, dingerUrl(r, key).String()})
}
