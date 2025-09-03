package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FriendEntry struct {
	AccountID string  `bson:"accountId" json:"accountId"`
	Created   string  `bson:"created" json:"created"`
	Alias     *string `bson:"alias,omitempty" json:"alias,omitempty"`
}

type FriendList struct {
	Accepted []FriendEntry `bson:"accepted" json:"accepted"`
	Incoming []FriendEntry `bson:"incoming" json:"incoming"`
	Outgoing []FriendEntry `bson:"outgoing" json:"outgoing"`
	Blocked  []FriendEntry `bson:"blocked" json:"blocked"`
}

type Friends struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AccountID string             `bson:"accountId" json:"accountId"`
	List      FriendList         `bson:"list" json:"list"`
}
