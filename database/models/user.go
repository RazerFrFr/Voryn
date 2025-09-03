package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Arena struct {
	Division int `bson:"division" json:"division"`
	Hype     int `bson:"hype" json:"hype"`
}

type User struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Created         time.Time          `bson:"created" json:"created"`
	Banned          bool               `bson:"banned" json:"banned"`
	AccountID       string             `bson:"accountId" json:"accountId"`
	Username        string             `bson:"username" json:"username"`
	Email           string             `bson:"email" json:"email"`
	Password        string             `bson:"password" json:"password"`
	MatchmakingID   string             `bson:"matchmakingId" json:"matchmakingId"`
	IsServer        bool               `bson:"isServer" json:"isServer"`
	AcceptedEULA    bool               `bson:"acceptedEULA" json:"acceptedEULA"`
	LastVbucksClaim time.Time          `bson:"lastVbucksClaim" json:"lastVbucksClaim"`
	Arena           Arena              `bson:"arena" json:"arena"`
}
