package humandao

import (
	"fmt"

	"cloud.google.com/go/firestore"
)

func convertReactionDocs(docs []*firestore.DocumentSnapshot) ([]Reaction, error) {
	reactions := make([]Reaction, 0, len(docs))
	for _, doc := range docs {
		reaction, err := convertReactionDoc(doc)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

func convertReactionDoc(doc *firestore.DocumentSnapshot) (Reaction, error) {
	var reaction Reaction
	if err := doc.DataTo(&reaction); err != nil {
		return Reaction{}, fmt.Errorf("unable to convert document to reaction: %w", err)
	}
	reaction.ID = doc.Ref.ID
	return reaction, nil
}

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
