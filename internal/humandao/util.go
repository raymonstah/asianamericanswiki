package humandao

import (
	"fmt"

	"cloud.google.com/go/firestore"
)

func convertHumansDocs(docs []*firestore.DocumentSnapshot) ([]Human, error) {
	humans := make([]Human, 0, len(docs))
	for _, doc := range docs {
		human, err := convertHumanDoc(doc)
		if err != nil {
			return nil, err
		}
		humans = append(humans, human)
	}

	return humans, nil
}

func convertHumanDoc(doc *firestore.DocumentSnapshot) (Human, error) {
	var human Human
	if err := doc.DataTo(&human); err != nil {
		return Human{}, fmt.Errorf("unable to convert document to human: %w", err)
	}
	human.ID = doc.Ref.ID
	return human, nil
}
