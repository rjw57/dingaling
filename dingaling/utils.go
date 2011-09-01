package dingaling

import (
	"appengine"
	"http"
	"json"
	"os"
)

// The function type which is wrapped by wrapHandler
type wrappedHandlerFunc func(appengine.Context, http.ResponseWriter, *http.Request) os.Error

// Wrap a handler so that it may return os.Error and have the error presented to the user as a HTTP Internal
// Server Error and logged to the admin console.
func wrapHandler(handleFunc wrappedHandlerFunc) http.HandlerFunc {
	f := func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)

		if err := handleFunc(c, w, r); err != nil {
			// QUESTION: I _believe_ err need not be sanitised. Is that right?
			http.Error(w, err.String(), http.StatusInternalServerError)

			// log the error
			c.Errorf("%v", err)
		}
	}
	return f
}

// Marshal v as a JSON object, set the Content-Type header and write the response.
func jsonResponse(w http.ResponseWriter, v interface{}) os.Error {
	w.Header().Set("Content-Type", "text/javascript") // yes, really!

	// Encode the value
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Write the raw bytes
	_, err = w.Write(jsonBytes)

	// Return the error (if any)
	return err
}
