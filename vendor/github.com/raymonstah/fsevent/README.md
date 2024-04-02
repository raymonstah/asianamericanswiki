fsevent
====

Eazy handling Cloud Firestore events on Google Cloud Functions Go Runtime.

## Description

Cloud Functions events triggered by Cloud Firestore are somewhat complex to handle.  
GCP Documentation: [Google Cloud Firestore Triggers](https://cloud.google.com/functions/docs/calling/cloud-firestore)  
Receive fsevent as an event and you can use the same type definition for Cloud Firestore.

### features

* get event types - `TypeCreate` `TypeUpdate` or `TypeDelete`
* reflect value in event to struct you defined for firestore

## Installation

Minimum Go version: Go 1.11

Use [go get](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) to install and update:

```bash
$ go get -u github.com/ya5u/fsevent
```

## Usage

Example Cloud Functions Code is below

```go
package handler

import (
  "context"
  "time"

  "github.com/ya5u/fsevent"
)

// FsData is defined type for firestore as you like
type FsData struct {
  Name     string     `firestore:"name"`
  Age      int64      `firestore:"age"`
  Birthday *time.Time `firestore:"birthday"`
}

// Handler is the entry point of Cloud Functions triggered by firestore event
func Handler(ctx context.Context, e fsevent.FirestoreEvent) error {
  // get event type
  eventType := e.Type()

  // reflect updated value to struct you defined
  var updated FsData
  err := e.Value.DataTo(&updated)
  if err != nil {
    // error handling
  }

  // reflect old value to struct you defined
  var old FsData
  err = e.OldValue.DataTo(&old)
  if err != nil {
    // error handling
  }
}
```

## TODO

* [x] support primitive pointer types
* [x] support Arrays
* [x] support Maps
* [ ] support References
* [ ] implement method of Value type like firestore.DocumentSnapshot.Data

## Licence

[MIT](https://github.com/ya5u/fsevent/blob/master/LICENSE)

## Author

[ya5u](https://github.com/ya5u)
