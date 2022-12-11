package contributor

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	date := time.Date(2021, 05, 24, 0, 25, 29, 0, time.UTC)
	input := FrontMatterInput{
		Name:          "Bruce Lee",
		Date:          date,
		Aliases:       []string{"Young Dragon"},
		Dob:           time.Date(1940, 11, 27, 0, 0, 0, 0, time.UTC),
		Tags:          []string{"martial artist", "actor"},
		Website:       "https://brucelee.com",
		Ethnicity:     []string{"Chinese"},
		BirthLocation: "San Francisco",
		Location:      []string{""},
		Twitter:       "",
		Draft:         false,
	}

	description := "Bruce Lee was a Chinese Martial Artist who was born in San Francisco, but returned to Hong Kong as a baby to live out his childhood years. In 1959, Bruce Lee returned to the states due to his juvenile troubles in Hong Kong. After studying philosophy and drama in college, he moved to Oakland to continue his Martial Arts practice professionally. Shortly after, he entered Hollywood and introduced Kung Fu to the rest of the world."

	asBytes, err := GenerateMarkdown(input, description)
	if err != nil {
		t.Fatalf("error generating markdown: %s", err)
	}

	got := string(asBytes)
	expected := `---
title: Bruce Lee
date: 2021-05-24T00:25:29
aka: [Young Dragon]
dob: "1940-11-27"
tags: [martial artist, actor]
website: https://brucelee.com
ethnicity: [Chinese]
birthLocation: San Francisco
location: [""]
twitter: ""
draft: false
ai_generated: false
---

Bruce Lee was a Chinese Martial Artist who was born in San Francisco, but returned to Hong Kong as a baby to live out his childhood years. In 1959, Bruce Lee returned to the states due to his juvenile troubles in Hong Kong. After studying philosophy and drama in college, he moved to Oakland to continue his Martial Arts practice professionally. Shortly after, he entered Hollywood and introduced Kung Fu to the rest of the world.`
	if got != expected {
		t.Fatalf("generated output did not match expected: got: \n%s\nexpected:\n%s\n", got, expected)
	}

}
