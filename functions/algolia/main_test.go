package algolia

import (
	"encoding/json"
	"testing"

	"github.com/raymonstah/fsevent"
	"github.com/tj/assert"
)

var deleteEvent = `{
  "oldValue": {
    "createTime": "2023-03-04T01:10:29.942831Z",
    "fields": null,
    "name": "projects/asianamericans-wiki/databases/(default)/documents/humans/foobar",
    "updateTime": "2023-03-04T01:10:29.942831Z"
  },
  "value": {
    "createTime": "0001-01-01T00:00:00Z",
    "fields": null,
    "name": "",
    "updateTime": "0001-01-01T00:00:00Z"
  },
  "updateMask": {
    "fieldPaths": null
  }
}`

var createEvent = `{
  "oldValue": {
    "createTime": "0001-01-01T00:00:00Z",
    "fields": null,
    "name": "",
    "updateTime": "0001-01-01T00:00:00Z"
  },
  "value": {
    "createTime": "2023-03-04T01:10:29.942831Z",
    "fields": null,
    "name": "projects/asianamericans-wiki/databases/(default)/documents/humans/foobar",
    "updateTime": "2023-03-04T01:10:29.942831Z"
  },
  "updateMask": {
    "fieldPaths": null
  }
}`

func TestGetHumanID(t *testing.T) {
	tcs := map[string]struct {
		event, want string
	}{
		"create": {event: createEvent, want: "foobar"},
		"delete": {event: deleteEvent, want: "foobar"},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			var event fsevent.FirestoreEvent
			err := json.Unmarshal([]byte(tc.event), &event)
			assert.NoError(t, err)

			gotHumanID := getHumanID(event)
			assert.Equal(t, tc.want, gotHumanID)
		})
	}
}

var updateEvent = `{
  "oldValue": {
    "createTime": "2022-08-13T22:07:01.241901Z",
    "fields": {},
    "name": "projects/asianamericans-wiki/databases/(default)/documents/humans/2DJsFGM3tqNV8axlFxfNtbWjyis",
    "updateTime": "2023-03-04T02:22:39.675775Z"
  },
  "value": {
    "createTime": "2022-08-13T22:07:01.241901Z",
    "fields": {
      "draft": {
        "booleanValue": true
      },
      "birth_location": {
        "stringValue": "Pflugerville, Texas"
      },
      "created_at": {
        "timestampValue": "2021-07-06T19:05:03Z"
      },
      "description": {
        "stringValue": "\n\nEugune is known for being a part of the YouTube group, The Try Guys. During the\nCOVID-19 pandemic, he has released a video on Anti-Asian Hate, which raised over\n$140,000 dollars in donations. The video can be viewed below:\n\n{{\u003c youtube 14WUuya94QE \u003e}}\n"
      },
      "dob": {
        "stringValue": "1986-01-18"
      },
      "ethnicity": {
        "arrayValue": {
          "values": [
            {
              "stringValue": "Korean"
            }
          ]
        }
      },
      "location": {
        "arrayValue": {
          "values": [
            {
              "stringValue": "Los Angeles"
            }
          ]
        }
      },
      "name": {
        "stringValue": "Eugene Lee Yang"
      },
      "tags": {
        "arrayValue": {
          "values": [
            {
              "stringValue": "writer"
            },
            {
              "stringValue": "youtuber"
            },
            {
              "stringValue": "director"
            },
            {
              "stringValue": "actor"
            },
            {
              "stringValue": "producer"
            },
            {
              "stringValue": "lgbt"
            }
          ]
        }
      },
      "twitter": {
        "stringValue": "https://twitter.com/EugeneLeeYang"
      },
      "updated_at": {
        "timestampValue": "2023-01-15T19:01:05.990Z"
      },
      "urn_path": {
        "stringValue": "eugene-lee-yang"
      }
    },
    "name": "projects/asianamericans-wiki/databases/(default)/documents/humans/foobar",
    "updateTime": "2023-03-04T02:27:21.851535Z"
  },
  "updateMask": {
    "fieldPaths": [
      "updated_at"
    ]
  }
}`

func TestFirestoreEvent(t *testing.T) {
	var event fsevent.FirestoreEvent
	err := json.Unmarshal([]byte(updateEvent), &event)
	assert.NoError(t, err)

	var human Human
	err = event.Value.DataTo(&human)
	assert.Equal(t, []string{"writer", "youtuber", "director", "actor", "producer", "lgbt"}, human.Tags)
	assert.Equal(t, "Eugene Lee Yang", human.Name)
	assert.True(t, human.Draft)
	assert.NoError(t, err)
}
