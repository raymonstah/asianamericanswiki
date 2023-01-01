package humandao

import "cloud.google.com/go/firestore"

type DAO struct {
	client             *firestore.Client
	humanCollection    string
	reactionCollection string
}

type Option func(d *DAO)

func WithHumanCollectionName(name string) Option {
	return func(d *DAO) {
		d.humanCollection = name
	}
}

func WithReactionCollectionName(name string) Option {
	return func(d *DAO) {
		d.reactionCollection = name
	}
}

func NewDAO(client *firestore.Client, options ...Option) *DAO {
	dao := &DAO{
		client:             client,
		humanCollection:    "humans",
		reactionCollection: "reactions",
	}

	for _, opt := range options {
		opt(dao)
	}

	return dao
}
