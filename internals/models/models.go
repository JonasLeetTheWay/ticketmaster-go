package models

import (
	"time"

	"gorm.io/gorm"
)

type Venue struct {
	gorm.Model
	Location string `gorm:"not null"` // required
	SeatMap  string
	Capacity int `gorm:"not null"`
}

type Performer struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	Genre       string
}

type Event struct {
	gorm.Model
	VenueID     uint `gorm:"not null"`
	PerformerID uint `gorm:"not null"`
	Name        string
	Description string
	Date        time.Time `gorm:"not null"`

	// Relationships
	Venue     Venue     `gorm:"foreignKey:VenueID"`
	Performer Performer `gorm:"foreignKey:PerformerID"`
	Tickets   []Ticket  `gorm:"foreignKey:EventID"`
}

type Ticket struct {
	gorm.Model
	EventID uint    `gorm:"not null"`
	Seat    string  `gorm:"not null"`
	Price   float64 `gorm:"not null"`
	Status  string  `gorm:"not null;default:'available'"`
	UserID  *uint
}

type User struct {
	gorm.Model
	Email    string `gorm:"not null;unique"`
	Password string `json:"-" gorm:"not null"`
	Name     string `gorm:"not null"`
}

type Booking struct {
	gorm.Model
	TicketID   uint `gorm:"not null"`
	UserID     uint `gorm:"not null"`
	Status     string
	ReservedAt time.Time
	ExpiresAt  time.Time
	PaymentID  uint `gorm:"not null"`
}

type ElasticsearchEvent struct {
	gorm.Model
	VenueID          uint
	PerformerID      uint
	Name             string
	Description      string
	Date             string
	Venue            string
	Performer        string
	Genre            string
	Location         string
	MinPrice         float64
	MaxPrice         float64
	AvailableTickets int
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
			&Venue{},
			&Performer{},
			&Event{},
			&Ticket{},
			&User{},
			&Booking{},
	)
}
