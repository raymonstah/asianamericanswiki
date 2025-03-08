package humandao

import "cloud.google.com/go/firestore"

type DAO struct {
	client          *firestore.Client
	humanCollection string
}

type Option func(d *DAO)

func WithHumanCollectionName(name string) Option {
	return func(d *DAO) {
		d.humanCollection = name
	}
}

func NewDAO(client *firestore.Client, options ...Option) *DAO {
	dao := &DAO{
		client:          client,
		humanCollection: "humans",
	}

	for _, opt := range options {
		opt(dao)
	}

	return dao
}
