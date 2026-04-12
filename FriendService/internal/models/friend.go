package models

type Friend struct {
	ID int `json:"id"`
}

type FriendRequest struct {
	FromUserID int    `json:"fromUserId"`
	ToUserID   int    `json:"toUserId"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

type FriendRecommendation struct {
	UserID        int `json:"userId"`
	MutualFriends int `json:"mutualFriends"`
}
