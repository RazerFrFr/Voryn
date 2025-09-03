package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TokenStore struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AccountID    string             `bson:"accountId" json:"accountId"`
	AccessToken  string             `bson:"accessToken" json:"accessToken"`
	RefreshToken string             `bson:"refreshToken" json:"refreshToken"`
	CreatedAt    time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}
