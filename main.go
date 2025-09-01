package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Constants
const (
	SECRET_KEY                = "restaurant_secret_key_2024"
	ACCESS_TOKEN_EXPIRE_HOURS = 999999
	da                        = "7609705273:AAFoIawJBTGTFxECwhSjc7vpbgMBcveT_ko"
	TELEGRAM_GROUP_ID         = "-1002783983140"
	UPLOAD_DIR                = "uploads"
	MAX_FILE_SIZE             = 10 << 20 // 10MB
	DATA_DIR                  = "data"
)

// JSON file paths
const (
	USERS_FILE    = "data/users.json"
	FOODS_FILE    = "data/foods.json"
	ORDERS_FILE   = "data/orders.json"
	REVIEWS_FILE  = "data/reviews.json"
	FILES_FILE    = "data/files.json"
	COUNTERS_FILE = "data/counters.json"
)

// Global data storage with mutex for thread safety
var (
	users    = make(map[string]*User)
	foods    = make(map[string]*Food) // String ID bilan
	orders   = make(map[string]*Order)
	reviews  = make(map[string]*Review)
	files    = make(map[string]*FileUpload)
	counters = &Counters{}

	// Mutexes for thread safety
	usersMutex    sync.RWMutex
	foodsMutex    sync.RWMutex
	ordersMutex   sync.RWMutex
	reviewsMutex  sync.RWMutex
	filesMutex    sync.RWMutex
	countersMutex sync.RWMutex
)

// Counters for auto-incrementing IDs
type Counters struct {
	FoodID   int64 `json:"food_id"`
	OrderNum int   `json:"order_num"`
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocket clients with mutex for thread safety
var (
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.RWMutex
	broadcast    = make(chan []byte)
)

// Enums
type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderConfirmed OrderStatus = "confirmed"
	OrderPreparing OrderStatus = "preparing"
	OrderReady     OrderStatus = "ready"
	OrderDelivered OrderStatus = "delivered"
	OrderCancelled OrderStatus = "cancelled"
)

type DeliveryType string

const (
	DeliveryHome       DeliveryType = "delivery"
	DeliveryPickup     DeliveryType = "own_withdrawal"
	DeliveryRestaurant DeliveryType = "atTheRestaurant"
)

type PaymentMethod string

const (
	PaymentCash  PaymentMethod = "cash"
	PaymentCard  PaymentMethod = "card"
	PaymentClick PaymentMethod = "click"
	PaymentPayme PaymentMethod = "payme"
)

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentPaid     PaymentStatus = "paid"
	PaymentFailed   PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
)

// Multi-language support for foods only
var FOOD_TRANSLATIONS = map[string]map[string]string{
	"uz": {
		"gazak":         "Sovuq Gazaklar",
		"salat":         "Salat",
		"zakuska":       "Issi Zakuskalar",
		"birinchi_taom": "Birinchi taomlar",
		"issiq_taom":    "Issiq taomlar",
		"baliq":         "Baliq",
		"shashlik":      "Shashlik",
		"garnir":        "Garnir",
		"non":           "Non",
		"choy":          "Choy",
		"kofe":          "Kofe",
		"moxito":        "Moxito",
		"ayran":         "Ayran",
		"ichimlik":      "Ichimliklar",
		"shirinlik":     "Shirinliklar",
		"bir_martalik":  "Bir martalik idishlar",
	},
	"ru": {
		"gazak":         "Холодные закуски",
		"salat":         "Салаты",
		"zakuska":       "Горячие Закуски",
		"birinchi_taom": "Первые блюда",
		"issiq_taom":    "Горячие блюда",
		"baliq":         "Рыба",
		"shashlik":      "Шашлык",
		"garnir":        "Гарниры",
		"non":           "Хлеб",
		"choy":          "Чай",
		"kofe":          "Кофе",
		"moxito":        "Мохито",
		"ayran":         "Айран",
		"ichimlik":      "Напитки",
		"shirinlik":     "Десерты",
		"bir_martalik":  "Одноразовая посуда",
	},
	"en": {
		"gazak":         "Appetizers",
		"salat":         "Salads",
		"zakuska":       "Hot appetizers",
		"birinchi_taom": "First courses",
		"issiq_taom":    "Hot dishes",
		"baliq":         "Fish",
		"shashlik":      "Barbecue",
		"garnir":        "Side dishes",
		"non":           "Bread",
		"choy":          "Tea",
		"kofe":          "Coffee",
		"moxito":        "Mojito",
		"ayran":         "Ayran",
		"ichimlik":      "Beverages",
		"shirinlik":     "Desserts",
		"bir_martalik":  "Disposable tableware",
	},
}

// Models
type User struct {
	ID        string    `json:"id"`
	Number    string    `json:"number"`
	Password  string    `json:"password,omitempty"`
	Role      string    `json:"role"`
	FullName  string    `json:"full_name"`
	Email     *string   `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
	TgID      *int64    `json:"tg_id,omitempty"`
	Language  string    `json:"language"`
}

type Food struct {
	ID              string              `json:"id"` // String ID
	Names           map[string]string   `json:"names,omitempty"`
	Name            string              `json:"name"`
	Descriptions    map[string]string   `json:"descriptions,omitempty"`
	Description     string              `json:"description"`
	Category        string              `json:"category"`
	CategoryName    string              `json:"category_name,omitempty"`
	Price           int                 `json:"price"`
	IsThere         bool                `json:"isThere"`
	ImageURL        string              `json:"imageUrl"`
	Ingredients     map[string][]string `json:"ingredients"`
	Allergens       map[string][]string `json:"allergens"`
	Rating          float64             `json:"rating"`
	ReviewCount     int                 `json:"review_count"`
	PreparationTime int                 `json:"preparation_time"`
	Stock           int                 `json:"stock"`
	IsPopular       bool                `json:"is_popular"`
	Discount        int                 `json:"discount"`
	OriginalPrice   int                 `json:"original_price"`
	Comment         string              `json:"comment"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type OrderFood struct {
	ID          string `json:"id"` // String ID
	Name        string `json:"name"`
	Category    string `json:"category"`
	Price       int    `json:"price"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Count       int    `json:"count"`
	TotalPrice  int    `json:"total_price"`
}

type PaymentInfo struct {
	Method        PaymentMethod `json:"method"`
	Status        PaymentStatus `json:"status"`
	Amount        int           `json:"amount"`
	TransactionID *string       `json:"transaction_id,omitempty"`
	PaymentTime   *time.Time    `json:"payment_time,omitempty"`
}

type DeliveryInfo struct {
	Type       string   `json:"type"`
	Address    *string  `json:"address,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Phone      *string  `json:"phone,omitempty"`
	TableID    *string  `json:"table_id,omitempty"`
	TableName  *string  `json:"table_name,omitempty"`
	PickupCode *string  `json:"pickup_code,omitempty"`
}

type Order struct {
	OrderID             string                 `json:"order_id"`
	UserNumber          string                 `json:"user_number"`
	UserName            string                 `json:"user_name"`
	Foods               []OrderFood            `json:"foods"`
	TotalPrice          int                    `json:"total_price"`
	OrderTime           time.Time              `json:"order_time"`
	DeliveryType        string                 `json:"delivery_type"`
	DeliveryInfo        map[string]interface{} `json:"delivery_info"`
	Status              OrderStatus            `json:"status"`
	PaymentInfo         PaymentInfo            `json:"payment_info"`
	SpecialInstructions *string                `json:"special_instructions,omitempty"`
	EstimatedTime       *int                   `json:"estimated_time,omitempty"`
	DeliveredAt         *time.Time             `json:"delivered_at,omitempty"`
	StatusHistory       []StatusUpdate         `json:"status_history,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

type StatusUpdate struct {
	Status    OrderStatus `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
	Note      string      `json:"note,omitempty"`
}

type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name,omitempty"`
	FoodID    string    `json:"food_id"` // String ID
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FileUpload struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	FileName     string    `json:"file_name"`
	FilePath     string    `json:"file_path"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	URL          string    `json:"url"`
	UploadedBy   string    `json:"uploaded_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// Request/Response structures
type LoginRequest struct {
	Number   string `json:"number" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Number   string  `json:"number" binding:"required"`
	Password string  `json:"password" binding:"required"`
	FullName string  `json:"full_name" binding:"required"`
	Email    *string `json:"email,omitempty"`
	TgID     *int64  `json:"tg_id,omitempty"`
	Language string  `json:"language,omitempty"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Role     string `json:"role"`
	UserID   string `json:"user_id"`
	Language string `json:"language"`
}

type FoodCreate struct {
	CustomID        *string  `json:"custom_id,omitempty"` // String ID
	NameUz          string   `json:"nameUz" binding:"required"`
	NameRu          string   `json:"nameRu" binding:"required"`
	NameEn          string   `json:"nameEn" binding:"required"`
	DescriptionUz   string   `json:"descriptionUz" binding:"required"`
	DescriptionRu   string   `json:"descriptionRu" binding:"required"`
	DescriptionEn   string   `json:"descriptionEn" binding:"required"`
	Category        string   `json:"category" binding:"required"`
	Price           int      `json:"price" binding:"required"`
	IsThere         bool     `json:"isThere"`
	ImageURL        string   `json:"imageUrl"`
	IngredientsUz   []string `json:"ingredientsUz,omitempty"`
	IngredientsRu   []string `json:"ingredientsRu,omitempty"`
	IngredientsEn   []string `json:"ingredientsEn,omitempty"`
	AllergensUz     []string `json:"allergensUz,omitempty"`
	AllergensRu     []string `json:"allergensRu,omitempty"`
	AllergensEn     []string `json:"allergensEn,omitempty"`
	PreparationTime int      `json:"preparation_time,omitempty"`
	Stock           int      `json:"stock,omitempty"`
	IsPopular       bool     `json:"is_popular,omitempty"`
	Discount        int      `json:"discount,omitempty"`
	Comment         string   `json:"comment,omitempty"`
	StarRating      float64  `json:"star_rating,omitempty"`
}

type CartItem struct {
	FoodID   string `json:"food_id" binding:"required"` // String ID
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

type OrderRequest struct {
	Items               []CartItem             `json:"items" binding:"required"`
	DeliveryType        DeliveryType           `json:"delivery_type" binding:"required"`
	DeliveryInfo        map[string]interface{} `json:"delivery_info"`
	PaymentMethod       PaymentMethod          `json:"payment_method" binding:"required"`
	SpecialInstructions *string                `json:"special_instructions,omitempty"`
	CustomerInfo        *CustomerInfo          `json:"customer_info,omitempty"`
}

type CustomerInfo struct {
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

type ReviewCreate struct {
	FoodID  string `json:"food_id" binding:"required"` // String ID
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"required"`
}

type ReviewUpdate struct {
	Rating  *int    `json:"rating,omitempty"`
	Comment *string `json:"comment,omitempty"`
}

// Telegram structures
type TelegramMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// JWT Claims
type Claims struct {
	Number string `json:"sub"`
	Role   string `json:"role"`
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Restaurant tables
var RestaurantTables = map[string]string{
	// Zal-1
	"Zal-1 Stol-1":  "93e05d01c3304b3b9dc963db187dbb51",
	"Zal-1 Stol-2":  "73d6827a734a43b6ad779b5979bb9c6a",
	"Zal-1 Stol-3":  "dc6e76e87f9e42a08a4e1198fc5f89a0",
	"Zal-1 Stol-4":  "70a53b0ac3264fce88d9a4b7d3a7fa5e",
	"Zal-1 Stol-5":  "3b8bfb57a10b4e4cb3b7a6d1434dd1bc",
	"Zal-1 Stol-6":  "4f0e0220e40b43b5a28747984474d6f7",
	"Zal-1 Stol-7":  "15fc7ed2ff3041aeaa52c5087e51f6b2",
	"Zal-1 Stol-8":  "41d0d60382b246469b7e01d70031c648",
	"Zal-1 Stol-9":  "539f421ed1974f55b86d09cfdace9ae3",
	"Zal-1 Stol-10": "1ad401f487024d1ab78e1db90eb3ac18",
	"Zal-1 Stol-11": "367f6587c09d4c1ebfe2b3e31c45b0ec",
	"Zal-1 Stol-12": "da2a9f108bff460aa1b3149b8fa9ed2a",
	"Zal-1 Stol-13": "91e91fa5a9e849aab850152b55613f98",
	"Zal-1 Stol-14": "d6d2ee01a57f4f4e93e6788eb1ccf4b2",
	"Zal-1 Stol-15": "b0f79bb99fef4492a26573f279845b9c",
	"Zal-1 Stol-16": "c2b7aeef8e814a9c8dfc4935cf8392f6",
	"Zal-1 Stol-17": "f4389cde50ac4c2ab4487a4a106d6d48",

	// Zal-2
	"Zal-2 Stol-1":  "c366a08ac9aa48d4a29f31de3561f69a",
	"Zal-2 Stol-2":  "d10a58dcb3cc4e3eb67a84f785a1a62d",
	"Zal-2 Stol-3":  "ecfc541124a54051b78e72930e1eac54",
	"Zal-2 Stol-4":  "e5baf1c7ed4d4a449fca1c7df1bb7006",
	"Zal-2 Stol-5":  "22bc7dbd17e145c6be40b1d01b29b16d",
	"Zal-2 Stol-6":  "ff6c4b82207f42a89b676ec5d0f1f7cc",
	"Zal-2 Stol-7":  "f00db03ddfa24d8b9f603a59cfb6f6cf",
	"Zal-2 Stol-8":  "f5c5bfa4a9974643b7a3aeb6d1114c7b",
	"Zal-2 Stol-9":  "62eb05a6882c401c953933132d43b7ff",
	"Zal-2 Stol-10": "bb842ff325a8498a99414958c400bc62",
	"Zal-2 Stol-11": "5ab7550a5ecf49b2b28faec156acbd44",
	"Zal-2 Stol-12": "9d640accb3d94fcbad09c191f03a7f8e",
	"Zal-2 Stol-13": "7a4044a32e2b4a35a9c91be98c3975a2",
	"Zal-2 Stol-14": "9c45db6ccda54e989f8b0ebf12c0a34b",
	"Zal-2 Stol-15": "f3fbbf2f179b4ec89745bfc3fdd10667",
	"Zal-2 Stol-16": "42134cd30da04d5b9e37fc68f7913fc7",

	// Terassa
	"Terassa Stol-1":  "3066c1f1c2e640e5a7272e28b4d08f8e",
	"Terassa Stol-2":  "5932a6769b154a94b7dbbf646e3725a3",
	"Terassa Stol-3":  "bc1dce5a12d049a489f5aa6f7aa64b3c",
	"Terassa Stol-4":  "a30c8e82ab6843d898c487ae9a6f31f2",
	"Terassa Stol-5":  "fa8e703e17924a99b4496c96459ae1e7",
	"Terassa Stol-6":  "32575a40ab784b878888b1de5421c24f",
	"Terassa Stol-7":  "f4530dcf98854f92a49d64b71b7d1372",
	"Terassa Stol-8":  "93c931e153694f69a9fd404be85727de",
	"Terassa Stol-9":  "4be17f7c57964e689d536cc946925e02",
	"Terassa Stol-10": "1ad9d8bbcc4e4b58b90ffed835f42e6b",
	"Terassa Stol-11": "49045b8e013d4722a72a41e3a5b8a761",
	"Terassa Stol-12": "f9a753a6bfc5483f9be02b36b3a021ae",
	"Terassa Stol-13": "c4a91adbf5c545f0b5c2cd0732e429ef",
	"Terassa Stol-14": "be6e16140c744418b47e021134a31b3f",
	"Terassa Stol-15": "c3c2317de56f4f8da8fa4c758dfb0427",
	"Terassa Stol-16": "76a5f6e3c08d4761b859ea0bb496fc63",

	// VIP stollar
	"VIP-1": "vip1_id_placeholder",
	"VIP-2": "vip2_id_placeholder",
	"VIP-3": "vip3_id_placeholder",
	"VIP-4": "vip4_id_placeholder",
	"VIP-5": "vip5_id_placeholder",
	"VIP-6": "vip6_id_placeholder",
	"VIP-7": "vip7_id_placeholder",
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	OrderID string      `json:"order_id,omitempty"`
}

// ========== JSON DATABASE FUNCTIONS ==========

func initJSONDatabase() error {
	// Create data directory if not exists
	if err := os.MkdirAll(DATA_DIR, 0755); err != nil {
		return fmt.Errorf("data directory creation error: %v", err)
	}

	// Initialize all JSON files
	if err := loadUsers(); err != nil {
		log.Printf("Loading users error: %v", err)
		return err
	}

	if err := loadFoods(); err != nil {
		log.Printf("Loading foods error: %v", err)
		return err
	}

	if err := loadOrders(); err != nil {
		log.Printf("Loading orders error: %v", err)
		return err
	}

	if err := loadReviews(); err != nil {
		log.Printf("Loading reviews error: %v", err)
		return err
	}

	if err := loadFiles(); err != nil {
		log.Printf("Loading files error: %v", err)
		return err
	}

	if err := loadCounters(); err != nil {
		log.Printf("Loading counters error: %v", err)
		return err
	}

	log.Println("✅ JSON Database initialized successfully")
	return nil
}

// Users JSON operations
func loadUsers() error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if _, err := os.Stat(USERS_FILE); os.IsNotExist(err) {
		// Create empty users file
		users = make(map[string]*User)
		return saveUsersUnsafe()
	}

	data, err := os.ReadFile(USERS_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		users = make(map[string]*User)
		return nil
	}

	return json.Unmarshal(data, &users)
}

func saveUsers() error {
	usersMutex.Lock()
	defer usersMutex.Unlock()
	return saveUsersUnsafe()
}

func saveUsersUnsafe() error {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(USERS_FILE, data, 0644)
}

// Foods JSON operations
func loadFoods() error {
	foodsMutex.Lock()
	defer foodsMutex.Unlock()

	if _, err := os.Stat(FOODS_FILE); os.IsNotExist(err) {
		// Create empty foods file
		foods = make(map[string]*Food)
		return saveFoodsUnsafe()
	}

	data, err := os.ReadFile(FOODS_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		foods = make(map[string]*Food)
		return nil
	}

	return json.Unmarshal(data, &foods)
}

func saveFoods() error {
	foodsMutex.Lock()
	defer foodsMutex.Unlock()
	return saveFoodsUnsafe()
}

func saveFoodsUnsafe() error {
	data, err := json.MarshalIndent(foods, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FOODS_FILE, data, 0644)
}

// Orders JSON operations
func loadOrders() error {
	ordersMutex.Lock()
	defer ordersMutex.Unlock()

	if _, err := os.Stat(ORDERS_FILE); os.IsNotExist(err) {
		// Create empty orders file
		orders = make(map[string]*Order)
		return saveOrdersUnsafe()
	}

	data, err := os.ReadFile(ORDERS_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		orders = make(map[string]*Order)
		return nil
	}

	return json.Unmarshal(data, &orders)
}

func saveOrders() error {
	ordersMutex.Lock()
	defer ordersMutex.Unlock()
	return saveOrdersUnsafe()
}

func saveOrdersUnsafe() error {
	data, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ORDERS_FILE, data, 0644)
}

// Reviews JSON operations
func loadReviews() error {
	reviewsMutex.Lock()
	defer reviewsMutex.Unlock()

	if _, err := os.Stat(REVIEWS_FILE); os.IsNotExist(err) {
		// Create empty reviews file
		reviews = make(map[string]*Review)
		return saveReviewsUnsafe()
	}

	data, err := os.ReadFile(REVIEWS_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		reviews = make(map[string]*Review)
		return nil
	}

	return json.Unmarshal(data, &reviews)
}

func saveReviews() error {
	reviewsMutex.Lock()
	defer reviewsMutex.Unlock()
	return saveReviewsUnsafe()
}

func saveReviewsUnsafe() error {
	data, err := json.MarshalIndent(reviews, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(REVIEWS_FILE, data, 0644)
}

// Files JSON operations
func loadFiles() error {
	filesMutex.Lock()
	defer filesMutex.Unlock()

	if _, err := os.Stat(FILES_FILE); os.IsNotExist(err) {
		// Create empty files file
		files = make(map[string]*FileUpload)
		return saveFilesUnsafe()
	}

	data, err := os.ReadFile(FILES_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		files = make(map[string]*FileUpload)
		return nil
	}

	return json.Unmarshal(data, &files)
}

func saveFiles() error {
	filesMutex.Lock()
	defer filesMutex.Unlock()
	return saveFilesUnsafe()
}

func saveFilesUnsafe() error {
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FILES_FILE, data, 0644)
}

// Counters JSON operations
func loadCounters() error {
	countersMutex.Lock()
	defer countersMutex.Unlock()

	if _, err := os.Stat(COUNTERS_FILE); os.IsNotExist(err) {
		// Create default counters
		counters = &Counters{
			FoodID:   1,
			OrderNum: 1,
		}
		return saveCountersUnsafe()
	}

	data, err := os.ReadFile(COUNTERS_FILE)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		counters = &Counters{
			FoodID:   1,
			OrderNum: 1,
		}
		return nil
	}

	return json.Unmarshal(data, counters)
}

func saveCounters() error {
	countersMutex.Lock()
	defer countersMutex.Unlock()
	return saveCountersUnsafe()
}

func saveCountersUnsafe() error {
	data, err := json.MarshalIndent(counters, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(COUNTERS_FILE, data, 0644)
}

// Helper functions for generating IDs
func getNextFoodID() int64 {
	countersMutex.Lock()
	defer countersMutex.Unlock()

	currentID := counters.FoodID
	counters.FoodID++
	saveCountersUnsafe() // Save immediately
	return currentID
}

func getNextOrderNum() int {
	countersMutex.Lock()
	defer countersMutex.Unlock()

	currentNum := counters.OrderNum
	counters.OrderNum++
	saveCountersUnsafe() // Save immediately
	return currentNum
}

// ========== UTILITY FUNCTIONS ==========

func getFoodTranslation(key, lang string) string {
	if lang == "" {
		lang = "ru"
	}
	if translations, exists := FOOD_TRANSLATIONS[lang]; exists {
		if translation, exists := translations[key]; exists {
			return translation
		}
	}
	// Default uzbek language
	if translations, exists := FOOD_TRANSLATIONS["ru"]; exists {
		if translation, exists := translations[key]; exists {
			return translation
		}
	}
	return key
}

func getUserLanguage(headers map[string][]string) string {
	acceptLang := headers["Accept-Language"]
	if len(acceptLang) > 0 {
		lang := strings.Split(acceptLang[0], ",")[0]
		if strings.Contains(lang, "-") {
			lang = strings.Split(lang, "-")[0]
		}
		lang = strings.ToLower(lang)
		supportedLangs := []string{"uz", "ru", "en"}
		for _, supported := range supportedLangs {
			if lang == supported {
				return lang
			}
		}
	}
	return "ru"
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.New().String()[:8])
}

func generateOrderID() string {
	today := time.Now().Format("2006-01-02")
	orderNum := getNextOrderNum()
	return fmt.Sprintf("%s-%d", today, orderNum)
}

func hashPassword(password string) string {
	hash := md5.Sum([]byte(password))
	return fmt.Sprintf("%x", hash)
}

func createToken(user *User) (string, error) {
	expirationTime := time.Now().Add(ACCESS_TOKEN_EXPIRE_HOURS * time.Hour)
	claims := &Claims{
		Number: user.Number,
		Role:   user.Role,
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(SECRET_KEY))
}

func getTableNameByID(tableID string) string {
	for tableName, id := range RestaurantTables {
		if id == tableID {
			return tableName
		}
	}
	return "Unknown table"
}

func getHostURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func cleanFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)
	cleaned := re.ReplaceAllString(name, "_")

	if len(cleaned) > 50 {
		ext := filepath.Ext(cleaned)
		nameWithoutExt := strings.TrimSuffix(cleaned, ext)
		if len(nameWithoutExt) > 46 {
			nameWithoutExt = nameWithoutExt[:46]
		}
		cleaned = nameWithoutExt + ext
	}

	return cleaned
}

// ========== FILE UPLOAD FUNCTIONS ==========

func uploadFile(c *gin.Context) {
	var uploaderID string
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		uploaderID = user.UserID
	}

	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file selected"})
		return
	}
	defer file.Close()

	if fileHeader.Size > MAX_FILE_SIZE {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
		return
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
		return
	}

	if _, err := os.Stat(UPLOAD_DIR); os.IsNotExist(err) {
		os.MkdirAll(UPLOAD_DIR, 0755)
	}

	originalName := strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
	cleanedName := cleanFileName(originalName)
	ext := filepath.Ext(fileHeader.Filename)

	fileName := fmt.Sprintf("%s_%d%s", cleanedName, time.Now().Unix(), ext)
	filePath := filepath.Join(UPLOAD_DIR, fileName)

	dst, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File save error"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File copy error"})
		return
	}

	fileURL := fmt.Sprintf("/uploads/%s", fileName)

	fileUpload := &FileUpload{
		ID:           generateID("file"),
		OriginalName: fileHeader.Filename,
		FileName:     fileName,
		FilePath:     filePath,
		FileSize:     fileHeader.Size,
		MimeType:     contentType,
		URL:          fileURL,
		UploadedBy:   uploaderID,
		CreatedAt:    time.Now(),
	}

	// Save to JSON
	filesMutex.Lock()
	files[fileUpload.ID] = fileUpload
	filesMutex.Unlock()

	if err := saveFiles(); err != nil {
		log.Printf("File data save error: %v", err)
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data save error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "File uploaded successfully",
		"file":       fileUpload,
		"url":        fileURL,
		"public_url": fileURL,
	})
}

// ========== TELEGRAM BOT FUNCTIONS ==========

func sendTelegramMessage(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", da)

	payload := TelegramMessage{
		ChatID: TELEGRAM_GROUP_ID,
		Text:   message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	return nil
}

func sendTelegramMessageToUser(userTgID int64, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", da)

	payload := TelegramMessage{
		ChatID: fmt.Sprintf("%d", userTgID),
		Text:   message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	return nil
}
func formatOrderForTelegram(order *Order) string {
	message := fmt.Sprintf("🍽️ Новый порядок!\n\n")
	message += fmt.Sprintf("📋 Идентификатор заказа: %s\n", order.OrderID)
	message += fmt.Sprintf("👤 Клиент: %s\n", order.UserName)
	message += fmt.Sprintf("📞 Телефон: +%s\n", order.UserNumber)
	message += fmt.Sprintf("🕐 Время: %s\n\n", order.OrderTime.Format("15:04"))

	message += fmt.Sprintf("🍕 Товары заказа:\n")
	for _, food := range order.Foods {
		message += fmt.Sprintf("• %s x%d\n", food.Name, food.Count)
	}

	// 💰 Narxni faqat "atTheRestaurant" bo'lmaganda qo'shamiz
	if order.DeliveryType != "atTheRestaurant" {
		message += fmt.Sprintf("\n💰 Total Amount: %d UZS\n", order.TotalPrice)
	}

	// Delivery information
	switch order.DeliveryType {
	case "delivery":
		if address, ok := order.DeliveryInfo["address"].(string); ok {
			message += fmt.Sprintf("🚚 Адрес доставки: %s\n", address)
		}
		if lat, ok := order.DeliveryInfo["latitude"].(float64); ok {
			if lng, ok := order.DeliveryInfo["longitude"].(float64); ok {
				message += fmt.Sprintf("📍 вызвать яндекс такси: https://taxi.yandex.uz/order?gfrom=39.7013104,67.0142258&gto=%.6f,%.6f&tariff=start&lang=uz&utm_source=yamaps&utm_medium=2334692&ref=233469\n", lat, lng)
			}
		} //https://taxi.yandex.uz/order?gfrom=39.7013104,67.0142258&gto=%.6f,%.6f&tariff=business&lang=uz&utm_source=yamaps&utm_medium=2334692&ref=233469
	case "own_withdrawal":
		message += fmt.Sprintf("🏪 Подобрать\n")
	case "atTheRestaurant":
		if tableName, ok := order.DeliveryInfo["table_name"].(string); ok {
			message += fmt.Sprintf("\n🍽️ Стол: %s\n", tableName)
		}
	}

	message += fmt.Sprintf("💳 Способ оплаты: %s\n", string(order.PaymentInfo.Method))

	if order.EstimatedTime != nil {
		message += fmt.Sprintf("⏱️ Время подготовки: %d минут\n", *order.EstimatedTime)
	}

	if order.SpecialInstructions != nil && *order.SpecialInstructions != "" {
		message += fmt.Sprintf("📝 Специальные инструкции: %s\n", *order.SpecialInstructions)
	}

	return message
}

// ========== WEBSOCKET FUNCTIONS (THREAD-SAFE VERSION) ==========

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Thread-safe client addition
	clientsMutex.Lock()
	clients[conn] = true
	totalClients := len(clients)
	clientsMutex.Unlock()

	log.Printf("WebSocket client connected. Total clients: %d", totalClients)

	// Read messages from client
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		// Handle incoming messages if needed
	}

	// Thread-safe client removal
	clientsMutex.Lock()
	delete(clients, conn)
	remainingClients := len(clients)
	clientsMutex.Unlock()

	log.Printf("WebSocket client disconnected. Remaining clients: %d", remainingClients)
}

func broadcastToClients(message WSMessage) {
	jsonData, _ := json.Marshal(message)

	clientsMutex.RLock()
	// Create a copy of clients to avoid holding the lock while writing
	clientsCopy := make([]*websocket.Conn, 0, len(clients))
	for client := range clients {
		clientsCopy = append(clientsCopy, client)
	}
	clientsMutex.RUnlock()

	// Send messages to clients without holding the lock
	var failedClients []*websocket.Conn
	for _, client := range clientsCopy {
		err := client.WriteJSON(message)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			failedClients = append(failedClients, client)
		}
	}

	// Remove failed clients
	if len(failedClients) > 0 {
		clientsMutex.Lock()
		for _, client := range failedClients {
			delete(clients, client)
		}
		clientsMutex.Unlock()
	}

	log.Printf("WebSocket message sent to %d clients: %s", len(clientsCopy)-len(failedClients), string(jsonData))
}

func sendOrderUpdate(orderID string, status OrderStatus, message string) {
	wsMessage := WSMessage{
		Type:    "order_update",
		OrderID: orderID,
		Data: gin.H{
			"order_id": orderID,
			"status":   status,
			"message":  message,
			"time":     time.Now(),
		},
	}
	broadcastToClients(wsMessage)
}

func sendNewOrderNotification(order *Order) {
	wsMessage := WSMessage{
		Type:    "new_order",
		OrderID: order.OrderID,
		Data:    order,
	}
	broadcastToClients(wsMessage)

	// Send Telegram message to admin group
	go func() {
		telegramMessage := formatOrderForTelegram(order)
		if err := sendTelegramMessage(telegramMessage); err != nil {
			log.Printf("Telegram message error: %v", err)
		} else {
			log.Printf("Telegram message sent: %s", order.OrderID)
		}
	}()
}

// ========== MIDDLEWARE ==========
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func optionalAuthMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString != authHeader {
				claims := &Claims{}
				token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(SECRET_KEY), nil
				})

				if err == nil && token.Valid {
					c.Set("user", claims)
				}
			}
		}
		c.Next()
	})
}

func authMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(SECRET_KEY), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user", claims)
		c.Next()
	})
}

func adminMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		claims := user.(*Claims)
		if claims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	})
}

// ========== DATABASE HELPER FUNCTIONS ==========

func getUserByNumber(number string) (*User, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	for _, user := range users {
		if user.Number == number {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func createUser(user *User) error {
	usersMutex.Lock()
	users[user.ID] = user
	usersMutex.Unlock()

	return saveUsers()
}

func getFoodByID(foodID string) (*Food, error) {
	foodsMutex.RLock()
	defer foodsMutex.RUnlock()

	if food, exists := foods[foodID]; exists {
		return food, nil
	}
	return nil, fmt.Errorf("food not found")
}

func getAllFoods() ([]*Food, error) {
	foodsMutex.RLock()
	defer foodsMutex.RUnlock()

	var result []*Food
	for _, food := range foods {
		if food.IsThere && food.Stock > 0 {
			result = append(result, food)
		}
	}

	// Sort by ID ascending (now ID is int)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func getAllFoodsForAdmin() ([]*Food, error) {
	foodsMutex.RLock()
	defer foodsMutex.RUnlock()

	var result []*Food
	for _, food := range foods {
		result = append(result, food)
	}

	// Sort by ID ascending (convert string to int for proper sorting)
	sort.Slice(result, func(i, j int) bool {
		idI, errI := strconv.ParseInt(result[i].ID, 10, 64)
		idJ, errJ := strconv.ParseInt(result[j].ID, 10, 64)

		// If both are numeric, sort numerically
		if errI == nil && errJ == nil {
			return idI < idJ
		}
		// Otherwise, sort lexically
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func createFoodWithCustomID(food *Food, customID *string) error {
	foodsMutex.Lock()
	defer foodsMutex.Unlock()

	if customID != nil && *customID != "" {
		// Check if custom ID already exists
		if _, exists := foods[*customID]; exists {
			return fmt.Errorf("food with ID %s already exists", *customID)
		}
		food.ID = *customID
	} else {
		// Use auto-incrementing ID (convert to string)
		nextID := getNextFoodID()
		food.ID = fmt.Sprintf("%d", nextID)
	}

	foods[food.ID] = food

	return saveFoodsUnsafe()
}

func updateFood(food *Food) error {
	foodsMutex.Lock()
	defer foodsMutex.Unlock()

	if _, exists := foods[food.ID]; !exists {
		return fmt.Errorf("food not found")
	}

	food.UpdatedAt = time.Now()
	foods[food.ID] = food

	return saveFoodsUnsafe()
}

func deleteFood(foodID string) error {
	foodsMutex.Lock()
	defer foodsMutex.Unlock()

	if _, exists := foods[foodID]; !exists {
		return fmt.Errorf("food not found")
	}

	delete(foods, foodID)

	return saveFoodsUnsafe()
}

func createOrder(order *Order) error {
	ordersMutex.Lock()
	orders[order.OrderID] = order
	ordersMutex.Unlock()

	return saveOrders()
}

func getOrderByID(orderID string) (*Order, error) {
	ordersMutex.RLock()
	defer ordersMutex.RUnlock()

	if order, exists := orders[orderID]; exists {
		return order, nil
	}
	return nil, fmt.Errorf("order not found")
}

func updateOrder(order *Order) error {
	ordersMutex.Lock()
	orders[order.OrderID] = order
	ordersMutex.Unlock()

	return saveOrders()
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func getLocalizedFood(food *Food, lang string) *Food {
	localizedFood := *food

	// Get multilingual name
	if food.Names != nil {
		if name, exists := food.Names[lang]; exists {
			localizedFood.Name = name
		} else if name, exists := food.Names["ru"]; exists {
			localizedFood.Name = name
		}
	}

	// Get multilingual description
	if food.Descriptions != nil {
		if desc, exists := food.Descriptions[lang]; exists {
			localizedFood.Description = desc
		} else if desc, exists := food.Descriptions["ru"]; exists {
			localizedFood.Description = desc
		}
	}

	// Translate category name
	categoryKey := strings.ToLower(strings.ReplaceAll(food.Category, " ", "_"))
	localizedFood.CategoryName = getFoodTranslation(categoryKey, lang)

	// Calculate discount
	if food.Discount > 0 {
		localizedFood.OriginalPrice = food.Price
		localizedFood.Price = food.Price - (food.Price * food.Discount / 100)
	}

	return &localizedFood
}

func getAllLocalizedFoods(lang string, isAdmin bool) ([]*Food, error) {
	var foods []*Food
	var err error

	if isAdmin {
		foods, err = getAllFoodsForAdmin()
	} else {
		foods, err = getAllFoods()
	}

	if err != nil {
		return nil, err
	}

	var localizedFoods []*Food
	for _, food := range foods {
		localizedFood := getLocalizedFood(food, lang)
		localizedFoods = append(localizedFoods, localizedFood)
	}

	return localizedFoods, nil
}

// ========== AUTHENTICATION HANDLERS ==========

func register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lang := req.Language
	if lang == "" {
		lang = "ru"
	}

	// Check if user exists
	_, err := getUserByNumber(req.Number)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number already registered"})
		return
	}

	user := &User{
		ID:        generateID("user"),
		Number:    req.Number,
		Password:  hashPassword(req.Password),
		Role:      "user",
		FullName:  req.FullName,
		Email:     req.Email,
		CreatedAt: time.Now(),
		IsActive:  true,
		TgID:      req.TgID,
		Language:  lang,
	}

	if err := createUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User creation error"})
		return
	}

	token, err := createToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token creation error"})
		return
	}

	response := LoginResponse{
		Token:    token,
		Role:     user.Role,
		UserID:   user.ID,
		Language: lang,
	}

	c.JSON(http.StatusOK, response)
}

func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := getUserByNumber(req.Number)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.Password != hashPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := createToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token creation error"})
		return
	}

	response := LoginResponse{
		Token:    token,
		Role:     user.Role,
		UserID:   user.ID,
		Language: user.Language,
	}

	c.JSON(http.StatusOK, response)
}

func getProfile(c *gin.Context) {
	user := c.MustGet("user").(*Claims)

	userDB, err := getUserByNumber(user.Number)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Remove password
	userResponse := *userDB
	userResponse.Password = ""

	c.JSON(http.StatusOK, userResponse)
}

// ========== CATEGORY HANDLERS ==========
func getCategories(c *gin.Context) {
	lang := getUserLanguage(c.Request.Header)

	categories := []gin.H{
		{"key": "gazak", "name": getFoodTranslation("gazak", lang)},
		{"key": "salat", "name": getFoodTranslation("salat", lang)},
		{"key": "zakuska", "name": getFoodTranslation("zakuska", lang)},
		{"key": "birinchi_taom", "name": getFoodTranslation("birinchi_taom", lang)},
		{"key": "issiq_taom", "name": getFoodTranslation("issiq_taom", lang)},
		{"key": "baliq", "name": getFoodTranslation("baliq", lang)},
		{"key": "shashlik", "name": getFoodTranslation("shashlik", lang)},
		{"key": "garnir", "name": getFoodTranslation("garnir", lang)},
		{"key": "non", "name": getFoodTranslation("non", lang)},
		{"key": "choy", "name": getFoodTranslation("choy", lang)},
		{"key": "kofe", "name": getFoodTranslation("kofe", lang)},
		{"key": "moxito", "name": getFoodTranslation("moxito", lang)},
		{"key": "ayran", "name": getFoodTranslation("ayran", lang)},
		{"key": "ichimlik", "name": getFoodTranslation("ichimlik", lang)},
		{"key": "shirinlik", "name": getFoodTranslation("shirinlik", lang)},
		{"key": "bir_martalik", "name": getFoodTranslation("bir_martalik", lang)},
	}

	c.JSON(http.StatusOK, categories)
}

// ========== FOOD HANDLERS ==========
// /

func getAllFoodsHandler(c *gin.Context) {
	lang := getUserLanguage(c.Request.Header)
	category := c.Query("category")
	search := c.Query("search")
	popular := c.Query("popular")
	sortBy := c.Query("sort")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	// Check if admin or regular user
	isAdmin := false
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		isAdmin = (user.Role == "admin")
	}

	foods, err := getAllLocalizedFoods(lang, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data fetch error"})
		return
	}

	// Filtering
	if category != "" {
		filtered := []*Food{}
		for _, food := range foods {
			if strings.ToLower(food.Category) == strings.ToLower(category) {
				filtered = append(filtered, food)
			}
		}
		foods = filtered
	}

	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := []*Food{}
		for _, food := range foods {
			if strings.Contains(strings.ToLower(food.Name), searchLower) ||
				strings.Contains(strings.ToLower(food.Description), searchLower) {
				filtered = append(filtered, food)
			}
		}
		foods = filtered
	}

	if popular == "true" {
		filtered := []*Food{}
		for _, food := range foods {
			if food.IsPopular {
				filtered = append(filtered, food)
			}
		}
		foods = filtered
	}

	// Sorting
	switch sortBy {
	case "price_asc":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].Price < foods[j].Price
		})
	case "price_desc":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].Price > foods[j].Price
		})
	case "rating":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].Rating > foods[j].Rating
		})
	case "popular":
		sort.Slice(foods, func(i, j int) bool {
			if foods[i].IsPopular != foods[j].IsPopular {
				return foods[i].IsPopular
			}
			return foods[i].Rating > foods[j].Rating
		})
	case "name":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].Name < foods[j].Name
		})
	default:
		// Default: ID ascending (sort numerically if possible)
		sort.Slice(foods, func(i, j int) bool {
			idI, errI := strconv.ParseInt(foods[i].ID, 10, 64)
			idJ, errJ := strconv.ParseInt(foods[j].ID, 10, 64)

			if errI == nil && errJ == nil {
				return idI < idJ
			}
			return foods[i].ID < foods[j].ID
		})
	}

	// Pagination
	total := len(foods)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		foods = []*Food{}
	} else {
		if end > total {
			end = total
		}
		foods = foods[start:end]
	}

	// Make ImageURL full URL
	hostURL := getHostURL(c)
	for _, food := range foods {
		if food.ImageURL != "" && !strings.HasPrefix(food.ImageURL, "http") {
			if strings.HasPrefix(food.ImageURL, "/") {
				food.ImageURL = hostURL + food.ImageURL
			} else {
				food.ImageURL = hostURL + "/" + food.ImageURL
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"foods": foods,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

func getFoodHandler(c *gin.Context) {
	foodID := c.Param("food_id") // String ID
	lang := getUserLanguage(c.Request.Header)

	food, err := getFoodByID(foodID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Food not found"})
		return
	}

	localizedFood := getLocalizedFood(food, lang)

	// Make ImageURL full URL
	if localizedFood.ImageURL != "" && !strings.HasPrefix(localizedFood.ImageURL, "http") {
		localizedFood.ImageURL = getHostURL(c) + localizedFood.ImageURL
	}

	c.JSON(http.StatusOK, localizedFood)
}

func createFoodHandler(c *gin.Context) {
	var req FoodCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON format",
			"details": err.Error(),
		})
		return
	}

	log.Printf("Received food creation request: %+v", req)

	// Set defaults
	if req.PreparationTime == 0 {
		req.PreparationTime = 15
	}
	if req.Stock == 0 {
		req.Stock = 100
	}

	// Create multilingual names
	names := map[string]string{
		"uz": req.NameUz,
		"ru": req.NameRu,
		"en": req.NameEn,
	}

	// Create multilingual descriptions
	descriptions := map[string]string{
		"uz": req.DescriptionUz,
		"ru": req.DescriptionRu,
		"en": req.DescriptionEn,
	}

	// Create multilingual ingredients
	ingredients := map[string][]string{
		"uz": req.IngredientsUz,
		"ru": req.IngredientsRu,
		"en": req.IngredientsEn,
	}

	// Create multilingual allergens
	allergens := map[string][]string{
		"uz": req.AllergensUz,
		"ru": req.AllergensRu,
		"en": req.AllergensEn,
	}

	food := &Food{
		Names:           names,
		Name:            req.NameUz, // Default Uzbek
		Descriptions:    descriptions,
		Description:     req.DescriptionUz, // Default Uzbek
		Category:        req.Category,
		Price:           req.Price,
		IsThere:         req.IsThere,
		ImageURL:        req.ImageURL,
		Ingredients:     ingredients,
		Allergens:       allergens,
		Rating:          req.StarRating, // Use star_rating from request
		ReviewCount:     0,
		PreparationTime: req.PreparationTime,
		Stock:           req.Stock,
		IsPopular:       req.IsPopular,
		Discount:        req.Discount,
		Comment:         req.Comment,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	log.Printf("Food object created: %+v", food)

	if err := createFoodWithCustomID(food, req.CustomID); err != nil {
		log.Printf("Food creation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Food creation error",
			"details": err.Error(),
		})
		return
	}

	log.Printf("Food created successfully: ID=%s, Name=%s", food.ID, food.Name)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Food created successfully",
		"food":    food,
		"id_type": map[bool]string{true: "custom", false: "auto"}[req.CustomID != nil],
	})
}

func updateFoodHandler(c *gin.Context) {
	foodID := c.Param("food_id") // String ID

	log.Printf("🔍 UPDATE REQUEST: Food ID = %s", foodID)

	food, err := getFoodByID(foodID)
	if err != nil {
		log.Printf("❌ Food not found: %s", foodID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Food not found"})
		return
	}

	log.Printf("📖 Current food before update: %+v", food)

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		log.Printf("❌ JSON bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📝 Received updates: %+v", updates)

	// Update fields with detailed logging
	if name, ok := updates["name"].(string); ok {
		log.Printf("🔄 Updating name: %s -> %s", food.Name, name)
		food.Name = name
	}

	if nameUz, ok := updates["nameUz"].(string); ok {
		log.Printf("🔄 Updating nameUz: %s", nameUz)
		if food.Names == nil {
			food.Names = make(map[string]string)
		}
		food.Names["uz"] = nameUz
	}

	if nameRu, ok := updates["nameRu"].(string); ok {
		log.Printf("🔄 Updating nameRu: %s", nameRu)
		if food.Names == nil {
			food.Names = make(map[string]string)
		}
		food.Names["ru"] = nameRu
	}

	if nameEn, ok := updates["nameEn"].(string); ok {
		log.Printf("🔄 Updating nameEn: %s", nameEn)
		if food.Names == nil {
			food.Names = make(map[string]string)
		}
		food.Names["en"] = nameEn
	}

	if category, ok := updates["category"].(string); ok {
		log.Printf("🔄 Updating category: %s -> %s", food.Category, category)
		food.Category = category
	}

	if price, ok := updates["price"].(float64); ok {
		log.Printf("🔄 Updating price: %d -> %d", food.Price, int(price))
		food.Price = int(price)
	}

	if description, ok := updates["description"].(string); ok {
		log.Printf("🔄 Updating description: %s -> %s", food.Description, description)
		food.Description = description
	}

	if descriptionUz, ok := updates["descriptionUz"].(string); ok {
		log.Printf("🔄 Updating descriptionUz: %s", descriptionUz)
		if food.Descriptions == nil {
			food.Descriptions = make(map[string]string)
		}
		food.Descriptions["uz"] = descriptionUz
	}

	if descriptionRu, ok := updates["descriptionRu"].(string); ok {
		log.Printf("🔄 Updating descriptionRu: %s", descriptionRu)
		if food.Descriptions == nil {
			food.Descriptions = make(map[string]string)
		}
		food.Descriptions["ru"] = descriptionRu
	}

	if descriptionEn, ok := updates["descriptionEn"].(string); ok {
		log.Printf("🔄 Updating descriptionEn: %s", descriptionEn)
		if food.Descriptions == nil {
			food.Descriptions = make(map[string]string)
		}
		food.Descriptions["en"] = descriptionEn
	}

	if isThere, ok := updates["isThere"].(bool); ok {
		log.Printf("🔄 Updating isThere: %v -> %v", food.IsThere, isThere)
		food.IsThere = isThere
	}

	if imageURL, ok := updates["imageUrl"].(string); ok {
		log.Printf("🔄 Updating imageUrl: %s -> %s", food.ImageURL, imageURL)
		food.ImageURL = imageURL
	}

	if prepTime, ok := updates["preparation_time"].(float64); ok {
		log.Printf("🔄 Updating preparation_time: %d -> %d", food.PreparationTime, int(prepTime))
		food.PreparationTime = int(prepTime)
	}

	if stock, ok := updates["stock"].(float64); ok {
		log.Printf("🔄 Updating stock: %d -> %d", food.Stock, int(stock))
		food.Stock = int(stock)
	}

	if isPopular, ok := updates["is_popular"].(bool); ok {
		log.Printf("🔄 Updating is_popular: %v -> %v", food.IsPopular, isPopular)
		food.IsPopular = isPopular
	}

	if discount, ok := updates["discount"].(float64); ok {
		log.Printf("🔄 Updating discount: %d -> %d", food.Discount, int(discount))
		food.Discount = int(discount)
	}

	if comment, ok := updates["comment"].(string); ok {
		log.Printf("🔄 Updating comment: %s -> %s", food.Comment, comment)
		food.Comment = comment
	}

	if rating, ok := updates["star_rating"].(float64); ok {
		log.Printf("🔄 Updating rating: %f -> %f", food.Rating, rating)
		food.Rating = rating
	}

	// Handle ingredients
	if ingredientsUz, ok := updates["ingredientsUz"].([]interface{}); ok {
		log.Printf("🔄 Updating ingredientsUz")
		if food.Ingredients == nil {
			food.Ingredients = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range ingredientsUz {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Ingredients["uz"] = strSlice
	}

	if ingredientsRu, ok := updates["ingredientsRu"].([]interface{}); ok {
		log.Printf("🔄 Updating ingredientsRu")
		if food.Ingredients == nil {
			food.Ingredients = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range ingredientsRu {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Ingredients["ru"] = strSlice
	}

	if ingredientsEn, ok := updates["ingredientsEn"].([]interface{}); ok {
		log.Printf("🔄 Updating ingredientsEn")
		if food.Ingredients == nil {
			food.Ingredients = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range ingredientsEn {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Ingredients["en"] = strSlice
	}

	// Handle allergens
	if allergensUz, ok := updates["allergensUz"].([]interface{}); ok {
		log.Printf("🔄 Updating allergensUz")
		if food.Allergens == nil {
			food.Allergens = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range allergensUz {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Allergens["uz"] = strSlice
	}

	if allergensRu, ok := updates["allergensRu"].([]interface{}); ok {
		log.Printf("🔄 Updating allergensRu")
		if food.Allergens == nil {
			food.Allergens = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range allergensRu {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Allergens["ru"] = strSlice
	}

	if allergensEn, ok := updates["allergensEn"].([]interface{}); ok {
		log.Printf("🔄 Updating allergensEn")
		if food.Allergens == nil {
			food.Allergens = make(map[string][]string)
		}
		var strSlice []string
		for _, v := range allergensEn {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		food.Allergens["en"] = strSlice
	}

	food.UpdatedAt = time.Now()

	log.Printf("📖 Food after updates: %+v", food)

	// Save to database
	log.Printf("💾 Saving food to database...")
	if err := updateFood(food); err != nil {
		log.Printf("❌ Food update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Food update error"})
		return
	}

	log.Printf("✅ Food updated successfully: ID=%s", food.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Food updated successfully",
		"food":    food,
	})
}

func deleteFoodHandler(c *gin.Context) {
	foodID := c.Param("food_id") // String ID

	// Check if food exists
	_, err := getFoodByID(foodID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Food not found"})
		return
	}

	// Delete food
	if err := deleteFood(foodID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Food deletion error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food deleted successfully"})
}

// ========== ORDER HANDLERS ==========

func createOrderHandler(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// User authentication
	var user *Claims
	if userInterface, exists := c.Get("user"); exists {
		user = userInterface.(*Claims)
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Login required"})
		return
	}

	// Check cart
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}

	log.Printf("Creating order for user: %s, items count: %d", user.Number, len(req.Items))

	// Check foods and stock
	var orderedFoods []OrderFood
	totalPrice := 0
	totalPrepTime := 0

	for _, item := range req.Items {
		log.Printf("Processing food_id: %s, quantity: %d", item.FoodID, item.Quantity)

		food, err := getFoodByID(item.FoodID) // String ID
		if err != nil {
			log.Printf("Food not found error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Food not available",
				"food_id": item.FoodID,
				"details": err.Error(),
			})
			return
		}

		if !food.IsThere || food.Stock <= 0 {
			log.Printf("Food not available: isThere=%v, stock=%d", food.IsThere, food.Stock)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Food not available",
				"food_id": item.FoodID,
			})
			return
		}

		// Check stock
		if food.Stock < item.Quantity {
			log.Printf("Insufficient stock: required=%d, available=%d", item.Quantity, food.Stock)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":     "Insufficient stock",
				"food_id":   item.FoodID,
				"required":  item.Quantity,
				"available": food.Stock,
			})
			return
		}

		localizedFood := getLocalizedFood(food, "ru")
		foodTotalPrice := localizedFood.Price * item.Quantity
		prepTime := food.PreparationTime
		if prepTime > totalPrepTime {
			totalPrepTime = prepTime
		}

		orderedFood := OrderFood{
			ID:          food.ID, // String ID
			Name:        localizedFood.Name,
			Category:    localizedFood.CategoryName,
			Price:       localizedFood.Price,
			Description: localizedFood.Description,
			ImageURL:    localizedFood.ImageURL,
			Count:       item.Quantity,
			TotalPrice:  foodTotalPrice,
		}
		orderedFoods = append(orderedFoods, orderedFood)
		totalPrice += foodTotalPrice

		// Reduce stock
		food.Stock -= item.Quantity
		if err := updateFood(food); err != nil {
			log.Printf("Stock update error: %v", err)
		}
	}

	log.Printf("Order foods processed successfully, total_price: %d", totalPrice)

	// Delivery information
	deliveryInfo := make(map[string]interface{})
	switch req.DeliveryType {
	case DeliveryHome:
		address, addressOk := req.DeliveryInfo["address"].(string)
		if !addressOk || address == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Delivery address required"})
			return
		}

		deliveryInfo = map[string]interface{}{
			"type":    "delivery",
			"address": address,
		}

		if phone, ok := req.DeliveryInfo["phone"].(string); ok {
			deliveryInfo["phone"] = phone
		}

		if lat, ok := req.DeliveryInfo["latitude"].(float64); ok {
			deliveryInfo["latitude"] = lat
		}
		if lng, ok := req.DeliveryInfo["longitude"].(float64); ok {
			deliveryInfo["longitude"] = lng
		}

		totalPrepTime += 20 // delivery time
	case DeliveryPickup:
		deliveryInfo = map[string]interface{}{
			"type":        "own_withdrawal",
			"pickup_code": generateID("pickup"),
		}
	case DeliveryRestaurant:
		tableID, ok := req.DeliveryInfo["table_id"].(string)
		if !ok || tableID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Table ID required"})
			return
		}
		tableName := getTableNameByID(tableID)
		deliveryInfo = map[string]interface{}{
			"type":       "atTheRestaurant",
			"table_id":   tableID,
			"table_name": tableName,
		}
	}

	log.Printf("Delivery info prepared: %+v", deliveryInfo)

	// Payment information
	paymentInfo := PaymentInfo{
		Method: req.PaymentMethod,
		Status: PaymentPending,
		Amount: totalPrice,
	}

	if req.PaymentMethod != PaymentCash {
		transactionID := generateID("txn")
		paymentInfo.TransactionID = &transactionID
	}

	log.Printf("Payment info prepared: %+v", paymentInfo)

	// Create order
	orderID := generateOrderID()
	orderTime := time.Now()

	userDB, _ := getUserByNumber(user.Number)
	userName := "User"
	if userDB != nil {
		userName = userDB.FullName
	}

	// Use customer info
	if req.CustomerInfo != nil {
		if req.CustomerInfo.Name != "" {
			userName = req.CustomerInfo.Name
		}
	}

	log.Printf("Order ID generated: %s", orderID)

	order := &Order{
		OrderID:             orderID,
		UserNumber:          user.Number,
		UserName:            userName,
		Foods:               orderedFoods,
		TotalPrice:          totalPrice,
		OrderTime:           orderTime,
		DeliveryType:        string(req.DeliveryType),
		DeliveryInfo:        deliveryInfo,
		Status:              OrderPending,
		PaymentInfo:         paymentInfo,
		SpecialInstructions: req.SpecialInstructions,
		EstimatedTime:       &totalPrepTime,
		StatusHistory: []StatusUpdate{
			{
				Status:    OrderPending,
				Timestamp: orderTime,
				Note:      "Order created",
			},
		},
		CreatedAt: orderTime,
		UpdatedAt: orderTime,
	}

	log.Printf("Order object created, attempting to save to JSON...")

	if err := createOrder(order); err != nil {
		log.Printf("JSON error creating order: %v", err)

		// Rollback stock
		for _, item := range req.Items {
			if food, err := getFoodByID(item.FoodID); err == nil {
				food.Stock += item.Quantity
				updateFood(food)
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Order creation error",
			"details": err.Error(),
		})
		return
	}

	log.Printf("Order created successfully in JSON: %s", orderID)

	// Real-time update
	go func() {
		sendNewOrderNotification(order)
		sendOrderUpdate(orderID, OrderPending, "Order created")
	}()

	c.JSON(http.StatusCreated, gin.H{
		"order":          order,
		"message":        "Order created successfully",
		"estimated_time": totalPrepTime,
		"order_tracking": fmt.Sprintf("/api/orders/%s/track", orderID),
	})
}

func getOrdersHandler(c *gin.Context) {
	user := c.MustGet("user").(*Claims)
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	ordersMutex.RLock()
	defer ordersMutex.RUnlock()

	var allOrders []*Order
	for _, order := range orders {
		// Filter by user if not admin
		if user.Role != "admin" && order.UserNumber != user.Number {
			continue
		}

		// Filter by status
		if status != "" && string(order.Status) != status {
			continue
		}

		allOrders = append(allOrders, order)
	}

	// Sort by order time descending
	sort.Slice(allOrders, func(i, j int) bool {
		return allOrders[i].OrderTime.After(allOrders[j].OrderTime)
	})

	// Pagination
	total := len(allOrders)
	offset := (page - 1) * limit
	end := offset + limit

	if offset >= total {
		allOrders = []*Order{}
	} else {
		if end > total {
			end = total
		}
		allOrders = allOrders[offset:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": allOrders,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

func getOrderHandler(c *gin.Context) {
	orderID := c.Param("order_id")
	user := c.MustGet("user").(*Claims)

	order, err := getOrderByID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// User can only see their own orders
	if user.Role != "admin" && order.UserNumber != user.Number {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func updateOrderStatusHandler(c *gin.Context) {
	orderID := c.Param("order_id")

	var req struct {
		Status OrderStatus `json:"status" binding:"required"`
		Note   string      `json:"note,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := getOrderByID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Add to status history
	statusUpdate := StatusUpdate{
		Status:    req.Status,
		Timestamp: time.Now(),
		Note:      req.Note,
	}
	order.StatusHistory = append(order.StatusHistory, statusUpdate)
	order.Status = req.Status

	if req.Status == OrderDelivered {
		now := time.Now()
		order.DeliveredAt = &now
		// Confirm payment
		if order.PaymentInfo.Method == PaymentCash {
			order.PaymentInfo.Status = PaymentPaid
			order.PaymentInfo.PaymentTime = &now
		}
	}

	if err := updateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Order update error"})
		return
	}

	// Real-time update
	sendOrderUpdate(orderID, req.Status, "Order status updated")

	// Send Telegram message to user
	go func() {
		if userDB, err := getUserByNumber(order.UserNumber); err == nil && userDB.TgID != nil {
			// Rus tilidagi status tarjimalari
			var statusText string
			switch req.Status {
			case OrderPending:
				statusText = "В ожидании"
			case OrderConfirmed:
				statusText = "Подтвержден"
			case OrderPreparing:
				statusText = "Готовится"
			case OrderReady:
				statusText = "Готов к выдаче"
			case OrderDelivered:
				statusText = "Доставлен"
			case OrderCancelled:
				statusText = "Отменён"
			default:
				statusText = string(req.Status)
			}

			userMessage := fmt.Sprintf("📋 Заказ: %s\n📍 Статус: %s", order.OrderID, statusText)
			if err := sendTelegramMessageToUser(*userDB.TgID, userMessage); err != nil {
				log.Printf("Ошибка при отправке сообщения пользователю в Telegram: %v", err)
			} else {
				log.Printf("Сообщение пользователю отправлено в Telegram: %s", order.OrderID)
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Order status updated",
		"order":   order,
	})
}

// ========== SEARCH HANDLER ==========

func searchHandler(c *gin.Context) {
	query := c.Query("q")
	category := c.Query("category")
	lang := getUserLanguage(c.Request.Header)
	minPrice, _ := strconv.Atoi(c.Query("min_price"))
	maxPrice, _ := strconv.Atoi(c.Query("max_price"))
	minRating, _ := strconv.ParseFloat(c.Query("min_rating"), 64)

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter required"})
		return
	}

	// Check if admin or regular user
	isAdmin := false
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		isAdmin = (user.Role == "admin")
	}

	foods, err := getAllLocalizedFoods(lang, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data fetch error"})
		return
	}

	// Search
	searchLower := strings.ToLower(query)
	var results []*Food

	for _, food := range foods {
		// Search in name, description and ingredients
		if strings.Contains(strings.ToLower(food.Name), searchLower) ||
			strings.Contains(strings.ToLower(food.Description), searchLower) {
			results = append(results, food)
			continue
		}

		// Search in ingredients
		if food.Ingredients != nil {
			if ingredientsList, ok := food.Ingredients[lang]; ok {
				for _, ingredient := range ingredientsList {
					if strings.Contains(strings.ToLower(ingredient), searchLower) {
						results = append(results, food)
						break
					}
				}
			} else if ingredientsList, ok := food.Ingredients["ru"]; ok {
				for _, ingredient := range ingredientsList {
					if strings.Contains(strings.ToLower(ingredient), searchLower) {
						results = append(results, food)
						break
					}
				}
			}
		}
	}

	// Filters
	if category != "" {
		filtered := []*Food{}
		for _, food := range results {
			if food.Category == category {
				filtered = append(filtered, food)
			}
		}
		results = filtered
	}

	if minPrice > 0 || maxPrice > 0 {
		filtered := []*Food{}
		for _, food := range results {
			if (minPrice == 0 || food.Price >= minPrice) &&
				(maxPrice == 0 || food.Price <= maxPrice) {
				filtered = append(filtered, food)
			}
		}
		results = filtered
	}

	if minRating > 0 {
		filtered := []*Food{}
		for _, food := range results {
			if food.Rating >= minRating {
				filtered = append(filtered, food)
			}
		}
		results = filtered
	}

	// Make ImageURL full URL
	hostURL := getHostURL(c)
	for _, food := range results {
		if food.ImageURL != "" && !strings.HasPrefix(food.ImageURL, "http") {
			food.ImageURL = hostURL + food.ImageURL
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":    query,
		"language": lang,
		"results":  results,
		"total":    len(results),
		"filters": gin.H{
			"category":   category,
			"min_price":  minPrice,
			"max_price":  maxPrice,
			"min_rating": minRating,
		},
	})
}

// ========== STATISTICS HANDLER ==========

func getStatisticsHandler(c *gin.Context) {
	ordersMutex.RLock()
	foodsMutex.RLock()
	usersMutex.RLock()
	defer ordersMutex.RUnlock()
	defer foodsMutex.RUnlock()
	defer usersMutex.RUnlock()

	// Order statistics
	var totalOrders, pendingOrders, completedOrders, cancelledOrders, totalRevenue int
	var todayOrders, todayRevenue int

	today := time.Now().Format("2006-01-02")

	for _, order := range orders {
		totalOrders++

		if order.Status == OrderDelivered {
			completedOrders++
			totalRevenue += order.TotalPrice
		} else if order.Status == OrderCancelled {
			cancelledOrders++
		} else {
			pendingOrders++
		}

		// Today's statistics
		if order.OrderTime.Format("2006-01-02") == today {
			todayOrders++
			if order.Status == OrderDelivered {
				todayRevenue += order.TotalPrice
			}
		}
	}

	// Food statistics
	totalFoods := len(foods)
	popularFoods := 0
	for _, food := range foods {
		if food.IsPopular || food.Rating >= 4.0 {
			popularFoods++
		}
	}

	// User statistics
	totalUsers := len(users)

	c.JSON(http.StatusOK, gin.H{
		"total_orders":     totalOrders,
		"pending_orders":   pendingOrders,
		"completed_orders": completedOrders,
		"cancelled_orders": cancelledOrders,
		"total_revenue":    totalRevenue,
		"today_orders":     todayOrders,
		"today_revenue":    todayRevenue,
		"total_foods":      totalFoods,
		"total_users":      totalUsers,
		"popular_foods":    popularFoods,
	})
}

// ========== INITIALIZATION ==========

func initializeTestData() error {
	// Admin user
	adminUser := &User{
		ID:        generateID("user"),
		Number:    "770451119",
		Password:  hashPassword("samandar"),
		Role:      "admin",
		FullName:  "Samandar Admin",
		Email:     stringPtr("admin@restaurant.uz"),
		CreatedAt: time.Now(),
		IsActive:  true,
		TgID:      int64Ptr(1066137436),
		Language:  "ru",
	}

	// Check if user exists
	_, err := getUserByNumber(adminUser.Number)
	if err != nil {
		if err := createUser(adminUser); err != nil {
			log.Printf("Admin user creation error: %v", err)
		} else {
			log.Println("✅ Admin user created")
		}
	} else {
		log.Printf("✅ Admin user exists")
	}

	// Test user
	testUser := &User{
		ID:        generateID("user"),
		Number:    "998901234567",
		Password:  hashPassword("user123"),
		Role:      "user",
		FullName:  "Test User",
		Email:     stringPtr("user@test.uz"),
		CreatedAt: time.Now(),
		IsActive:  true,
		TgID:      int64Ptr(1066137436),
		Language:  "uz",
	}

	_, err = getUserByNumber(testUser.Number)
	if err != nil {
		if err := createUser(testUser); err != nil {
			log.Printf("Test user creation error: %v", err)
		} else {
			log.Println("✅ Test user created")
		}
	} else {
		log.Printf("✅ Test user exists")
	}

	// Create some sample foods if empty
	foodsMutex.RLock()
	foodsCount := len(foods)
	foodsMutex.RUnlock()

	if foodsCount == 0 {
		log.Println("Creating sample foods...")

		sampleFoods := []*Food{}

		for _, food := range sampleFoods {
			if err := createFoodWithCustomID(food, nil); err != nil {
				log.Printf("Sample food creation error: %v", err)
			}
		}

		log.Println("✅ Sample foods created")
	}

	return nil
}

// ========== ROOT HANDLER ==========

func rootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":             "Restaurant API - JSON Database Version (Thread-Safe)",
		"version":             "8.1.0",
		"supported_languages": []string{"uz", "ru", "en"},
		"features": []string{
			"JSON File Database System",
			"Thread-Safe Operations with Mutexes",
			"Thread-Safe WebSocket Connections",
			"String ID Support",
			"Auto-incrementing IDs converted to string",
			"Manual String ID Insertion Support",
			"File Upload with Food Names",
			"Telegram Bot Integration",
			"GPS Coordinates for Delivery",
			"Stock Management",
			"Real-time Order Tracking",
			"WebSocket Support (Fixed Concurrent Map Writes)",
			"Advanced Search",
			"Multi-language Food Support",
			"English Error Messages",
		},
		"endpoints": gin.H{
			"foods":       "/api/foods (PUBLIC)",
			"food_by_id":  "/api/foods/:id (PUBLIC - STRING ID)",
			"categories":  "/api/categories (PUBLIC)",
			"search":      "/api/search (PUBLIC)",
			"upload":      "/api/upload (PUBLIC/AUTH)",
			"orders":      "/api/orders (AUTH)",
			"websocket":   "/ws",
			"statistics":  "/api/admin/statistics (ADMIN)",
			"custom_food": "/api/admin/foods (ADMIN - supports string custom_id)",
		},
		"database": gin.H{
			"type":      "JSON Files",
			"status":    "connected",
			"id_system": "String IDs (auto: '1', '2', '3'... or custom: 'FOOD_001', 'custom_id')",
			"files": gin.H{
				"users":    USERS_FILE,
				"foods":    FOODS_FILE,
				"orders":   ORDERS_FILE,
				"reviews":  REVIEWS_FILE,
				"files":    FILES_FILE,
				"counters": COUNTERS_FILE,
			},
		},
		"integrations": gin.H{
			"telegram_bot":            "enabled",
			"user_notifications":      "enabled",
			"file_upload":             "enabled",
			"gps_tracking":            "enabled",
			"string_id_support":       "enabled",
			"custom_string_id":        "enabled",
			"json_persistence":        "enabled",
			"websocket_thread_safety": "enabled",
			"concurrent_map_fix":      "applied",
		},
		"fixes": gin.H{
			"concurrent_map_writes": "FIXED - Added mutex protection for WebSocket clients map",
			"thread_safety":         "ENHANCED - All map operations now thread-safe",
			"websocket_stability":   "IMPROVED - Proper connection lifecycle management",
		},
	})
}

// ========== SETUP ROUTES ==========

func setupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Middleware
	r.Use(corsMiddleware())

	// Static files
	if _, err := os.Stat(UPLOAD_DIR); os.IsNotExist(err) {
		os.MkdirAll(UPLOAD_DIR, 0755)
	}

	// Static file route
	r.Static("/uploads", "./"+UPLOAD_DIR)
	r.StaticFile("/favicon.ico", "./favicon.ico")

	// WebSocket endpoint
	r.GET("/ws", handleWebSocket)

	// Root endpoint
	r.GET("/", rootHandler)

	// API group
	api := r.Group("/api")

	// PUBLIC ENDPOINTS
	public := api.Group("/")
	{
		public.GET("/categories", getCategories)
		public.GET("/foods", optionalAuthMiddleware(), getAllFoodsHandler)
		public.GET("/foods/:food_id", getFoodHandler)
		public.GET("/search", optionalAuthMiddleware(), searchHandler)
		public.POST("/upload", optionalAuthMiddleware(), uploadFile)
	}

	// Authentication endpoints
	auth := api.Group("/")
	{
		auth.POST("/register", register)
		auth.POST("/login", login)
	}

	// Protected endpoints
	protected := api.Group("/")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", getProfile)
		protected.POST("/orders", createOrderHandler)
		protected.GET("/orders", getOrdersHandler)
		protected.GET("/orders/:order_id", getOrderHandler)
	}

	// Admin endpoints
	admin := protected.Group("/admin")
	admin.Use(adminMiddleware())
	{
		admin.POST("/foods", createFoodHandler) // Supports string custom_id
		admin.PUT("/foods/:food_id", updateFoodHandler)
		admin.DELETE("/foods/:food_id", deleteFoodHandler)
		admin.PUT("/orders/:order_id/status", updateOrderStatusHandler)
		admin.GET("/statistics", getStatisticsHandler)
		admin.POST("/reload-data", func(c *gin.Context) {
			if err := initJSONDatabase(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Data reloaded successfully"})
		})
	}

	return r
}

// ========== MAIN FUNCTION ==========

func main() {
	// JSON Database initialization
	if err := initJSONDatabase(); err != nil {
		log.Fatalf("❌ JSON Database error: %v", err)
	}

	// Test data initialization
	if err := initializeTestData(); err != nil {
		log.Printf("⚠️ Test data creation error: %v", err)
	}

	// WebSocket broadcast handler (thread-safe)
	go func() {
		for {
			select {
			case msg := <-broadcast:
				// Thread-safe broadcast
				clientsMutex.RLock()
				clientsCopy := make([]*websocket.Conn, 0, len(clients))
				for client := range clients {
					clientsCopy = append(clientsCopy, client)
				}
				clientsMutex.RUnlock()

				// Send to clients without holding the lock
				var failedClients []*websocket.Conn
				for _, client := range clientsCopy {
					if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
						log.Printf("WebSocket broadcast error: %v", err)
						client.Close()
						failedClients = append(failedClients, client)
					}
				}

				// Remove failed clients
				if len(failedClients) > 0 {
					clientsMutex.Lock()
					for _, client := range failedClients {
						delete(clients, client)
					}
					clientsMutex.Unlock()
				}
			}
		}
	}()

	// Server setup
	r := setupRoutes()

	// Server port
	port := os.Getenv("PORT")
	if port == "" {
		port = "3030"
	}

	// Get local IP address
	localIP := getLocalIP()

	log.Printf("🚀 Restaurant API - JSON Database Version (Thread-Safe WebSocket):")
	log.Printf("📍 Local Server: http://localhost:%s", port)
	log.Printf("🌐 WiFi Access: http://%s:%s", localIP, port)
	log.Printf("🔗 WebSocket Local: ws://localhost:%s/ws", port)
	log.Printf("🔗 WebSocket WiFi: ws://%s:%s/ws", localIP, port)
	log.Printf("📚 API Docs: http://%s:%s/", localIP, port)
	log.Printf("🍽️ Public Foods: http://%s:%s/api/foods", localIP, port)
	log.Printf("🔍 Search: http://%s:%s/api/search", localIP, port)
	log.Printf("📤 File Upload: http://%s:%s/api/upload", localIP, port)
	log.Printf("📊 Admin Stats: http://%s:%s/api/admin/statistics", localIP, port)
	log.Printf("🖼️ Static Files: http://%s:%s/uploads/", localIP, port)
	log.Printf("🔢 ID System: String IDs (auto: '1', '2', '3'... or custom: 'FOOD_001')")
	log.Printf("🎯 Manual ID: POST /api/admin/foods with string custom_id")
	log.Printf("🗄️ Database: JSON Files (%s)", DATA_DIR)
	log.Printf("🤖 Telegram Bot: Admin + User Notifications")
	log.Printf("📍 GPS: Delivery Coordinates Support")
	log.Printf("👁️ Visibility: Only available foods (isThere=true, stock>0)")
	log.Printf("🌐 Languages: Foods support UZ/RU/EN, Errors in English")
	log.Printf("🔒 Thread Safety: Mutex locks for all operations including WebSocket")
	log.Printf("📝 String IDs: Full support for custom string identifiers")
	log.Printf("📱 Mobile Access: Use WiFi IP address to access from other devices")
	log.Printf("⚠️ Firewall: Make sure port %s is open for incoming connections", port)
	log.Printf("🔧 FIXED: Concurrent map writes error in WebSocket handling")
	log.Printf("✅ WebSocket: Thread-safe connection management implemented")

	// Start server on all interfaces (0.0.0.0)
	log.Fatal(r.Run("0.0.0.0:" + port))
}

// getLocalIP returns the local IP address
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("⚠️ Cannot get local IP: %v", err)
		return "IP_NOT_FOUND"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// Alternative method to get all network interfaces
func getAllNetworkInterfaces() {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces: %v", err)
		return
	}

	log.Printf("📡 Available Network Interfaces:")
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						log.Printf("   • %s: %s", iface.Name, ipnet.IP.String())
					}
				}
			}
		}
	}
}
