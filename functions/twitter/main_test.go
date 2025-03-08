package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/raymonstah/fsevent"
	"github.com/tj/assert"
)

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
        "booleanValue": false
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

func TestParseHandles(t *testing.T) {
	testcases := map[string]struct {
		Raw            string
		ExpectedHandle string
	}{
		"basic-handle":  {Raw: `"@raymond"`, ExpectedHandle: "raymond"},
		"single-quotes": {Raw: `'https://twitter.com/raymond'`, ExpectedHandle: "raymond"},
		"double-quotes": {Raw: `"https://twitter.com/raymond"`, ExpectedHandle: "raymond"},
		"no-quotes":     {Raw: `https://twitter.com/raymond`, ExpectedHandle: "raymond"},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			got := parseHandle(testcase.Raw)
			if got != testcase.ExpectedHandle {
				t.Fatalf("expected %v, got %v", testcase.ExpectedHandle, got)
			}
		})
	}
}

func TestTwitterFollow(t *testing.T) {
	ctx := context.Background()
	var event fsevent.FirestoreEvent
	err := json.Unmarshal([]byte(updateEvent), &event)
	assert.NoError(t, err)
	var (
		apiKey       = os.Getenv("TWITTER_API_KEY")
		apiKeySecret = os.Getenv("TWITTER_API_KEY_SECRET")
		accessToken  = os.Getenv("TWITTER_ACCESS_TOKEN")
		accessSecret = os.Getenv("TWITTER_ACCESS_SECRET")
	)
	if apiKey == "" || apiKeySecret == "" || accessToken == "" || accessSecret == "" {
		t.SkipNow()
	}

	err = TwitterFollow(ctx, event)
	assert.NoError(t, err)
}
