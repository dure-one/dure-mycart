package models

// Sellerinfo holds the seller/business information displayed to customers.
// This is a dure-mycart extension for Korean e-commerce regulations.
type Sellerinfo struct {
	Enabled           bool   `json:"enabled"`
	BusinessName      string `json:"business_name"`
	Representative    string `json:"representative"`
	CustomerService   string `json:"customer_service"`
	BusinessRegNumber string `json:"business_reg_number"`
	BusinessAddress   string `json:"business_address"`
	EcommerceLicense  string `json:"ecommerce_license"`
	Email             string `json:"email"`
}
