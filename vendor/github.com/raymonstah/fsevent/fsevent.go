package fsevent

// event types
const (
	TypeCreate = "CREATE" // on a document created
	TypeUpdate = "UPDATE" // on a document updated
	TypeDelete = "DELETE" // on a document deleted
)

// FirestoreEvent is the payload of a Firestore event.
type FirestoreEvent struct {
	OldValue   *Value `json:"oldValue"`
	Value      *Value `json:"value"`
	UpdateMask *struct {
		FieldPaths []string `json:"fieldPaths"`
	} `json:"updateMask"`
}

// Type returns event type, which is one of following constants:
//   - TypeCreate
//   - TypeUpdate
//   - TypeDelete
func (e *FirestoreEvent) Type() string {
	if len(e.UpdateMask.FieldPaths) > 0 {
		return TypeUpdate
	}
	if len(e.Value.Name) > 0 {
		return TypeCreate
	}
	return TypeDelete
}
