package xai

import (
	"context"
	"os"
	"testing"

	"github.com/tj/assert"
)

func TestXAIFunction(t *testing.T) {
	token := os.Getenv("XAI_API_KEY")
	if token == "" {
		t.SkipNow()
	}

	client := New(token)
	ctx := context.Background()
	data := `
Jonathan Edgar Park[1] (born February 18, 1986),[2] known by his stage name Dumbfoundead (/ˈdʌmˌfaʊndɪd/[3]), is an Argentinian-born American rapper.[4] He began his career in the 2000s as a battle rapper in Los Angeles and has since become one of the most prominent East Asian American rappers, known for his witty and socially conscious lyrics.[5][6][7]

Early life
Park was born in Buenos Aires, Argentina, to South Korean immigrants. He has one younger sister. When he was three years old, Park's family immigrated to the United States by crossing the Mexico–United States border without green cards. His family settled in the Koreatown neighborhood of Los Angeles, California.[4]

Park began rapping when he was fourteen years old, inspired in part by the rappers he saw perform weekly at Project Blowed, a local open-microphone workshop.[8] He dropped out of John Marshall High School in his sophomore year and moved into a one-bedroom apartment with his sister and a roommate at the age of sixteen. Before becoming a full-time rapper, he worked as a bail bondsman, among other odd jobs.[8]

Park became a U.S. citizen when he was nineteen years old.[4]
`

	addHumanResponse, err := client.FromText(ctx, FromTextInput{Data: data})
	assert.NoError(t, err)
	assert.Equal(t, "male", addHumanResponse.Gender)
	assert.Equal(t, "1986-02-18", addHumanResponse.DOB)
	assert.Equal(t, "Jonathan Edgar Park", addHumanResponse.Name)
	assert.Empty(t, addHumanResponse.DOD)
	assert.NotEmpty(t, addHumanResponse.Description)
	assert.Contains(t, addHumanResponse.Ethnicity, "south korean")
}
