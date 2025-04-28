package models

type BookImage struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	BookID uint   `gorm:"index" json:"book_id"`
	URL    string `json:"url"`
}
