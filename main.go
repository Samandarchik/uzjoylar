package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	ACCESS_TOKEN_EXPIRE_HOURS = 24
	TELEGRAM_BOT_TOKEN        = "7609705273:AAH_CsC52AiiZCeZ828HaHzYgHKpJBvSLI0"
	TELEGRAM_GROUP_ID         = "-1002783983140"
	UPLOAD_DIR                = "uploads"
	DATA_DIR                  = "data"
	MAX_FILE_SIZE             = 10 << 20 // 10MB
)

// JSON filenames
const (
	USERS_FILE         = "users.json"
	FOODS_FILE         = "foods.json"
	ORDERS_FILE        = "orders.json"
	REVIEWS_FILE       = "reviews.json"
	FILE_UPLOADS_FILE  = "file_uploads.json"
	DAILY_COUNTER_FILE = "daily_counter.json"
)

// Mutex for file operations
var (
	usersLock   sync.RWMutex
	foodsLock   sync.RWMutex
	ordersLock  sync.RWMutex
	reviewsLock sync.RWMutex
	uploadsLock sync.RWMutex
	counterLock sync.RWMutex
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocket clients
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

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

// Translations
var TRANSLATIONS = map[string]map[string]string{
	"uz": {
		"success":                    "Muvaffaqiyatli",
		"error":                      "Xatolik",
		"not_found":                  "Topilmadi",
		"unauthorized":               "Ruxsat etilmagan",
		"forbidden":                  "Taqiqlangan",
		"invalid_request":            "Noto'g'ri so'rov",
		"phone_already_registered":   "Bu telefon raqami allaqachon ro'yxatdan o'tgan",
		"invalid_credentials":        "Telefon raqami yoki parol noto'g'ri",
		"user_not_found":             "Foydalanuvchi topilmadi",
		"token_invalid":              "Token yaroqsiz",
		"registration_successful":    "Ro'yxatdan o'tish muvaffaqiyatli",
		"login_successful":           "Tizimga kirish muvaffaqiyatli",
		"food_not_found":             "Ovqat topilmadi",
		"food_created":               "Ovqat yaratildi",
		"food_updated":               "Ovqat ma'lumotlari yangilandi",
		"food_deleted":               "Ovqat o'chirildi",
		"only_admin_can_manage_food": "Faqat admin ovqat boshqara oladi",
		"order_created":              "Buyurtma yaratildi",
		"order_not_found":            "Buyurtma topilmadi",
		"order_cancelled":            "Buyurtma bekor qilindi",
		"order_status_updated":       "Buyurtma holati yangilandi",
		"food_not_available":         "Ovqat mavjud emas",
		"invalid_quantity":           "Ovqat miqdori 0 dan katta bo'lishi kerak",
		"order_confirmed":            "Buyurtmangiz tasdiqlandi!",
		"new_order":                  "Yangi buyurtma!",
		"order_id":                   "Buyurtma ID:",
		"customer":                   "Mijoz:",
		"phone":                      "Telefon:",
		"time":                       "Vaqt:",
		"order_items":                "Buyurtma tarkibi:",
		"total_amount":               "Umumiy summa:",
		"delivery_address":           "Yetkazib berish:",
		"pickup":                     "O'zi olib ketish:",
		"restaurant_table":           "Restoranda:",
		"payment_method":             "To'lov usuli:",
		"preparation_time":           "Tayyorlash vaqti:",
		"additional_notes":           "Qo'shimcha:",
		"shashlik":                   "Shashlik",
		"milliy_taomlar":             "Milliy taomlar",
		"ichimliklar":                "Ichimliklar",
		"salatlar":                   "Salatlar",
		"shirinliklar":               "Shirinliklar",
		"delivery":                   "Yetkazib berish",
		"own_withdrawal":             "O'zi olib ketish",
		"at_restaurant":              "Restoranda",
		"cash":                       "Naqd",
		"card":                       "Karta",
		"click":                      "Click",
		"payme":                      "Payme",
		"login_required":             "Buyurtma berish uchun tizimga kiring",
		"cart_empty":                 "Savatda mahsulot yo'q",
		"insufficient_stock":         "Yetarli miqdor yo'q",
		"order_processing":           "Buyurtma qayta ishlanmoqda",
		"file_uploaded":              "Fayl yuklandi",
		"invalid_file":               "Noto'g'ri fayl",
		"file_too_large":             "Fayl hajmi katta",
		"order_status_pending":       "Buyurtmangiz qabul qilindi",
		"order_status_confirmed":     "Buyurtmangiz tasdiqlandi!",
		"order_status_preparing":     "Buyurtmangiz tayyorlanmoqda",
		"order_status_ready":         "Buyurtmangiz tayyor!",
		"order_status_delivered":     "Buyurtmangiz yetkazildi",
		"order_status_cancelled":     "Buyurtmangiz bekor qilindi",
		"review_created":             "Sharh qo'shildi",
		"review_updated":             "Sharh yangilandi",
		"review_deleted":             "Sharh o'chirildi",
	},
	"ru": {
		"success":                    "Успешно",
		"error":                      "Ошибка",
		"not_found":                  "Не найдено",
		"unauthorized":               "Неавторизован",
		"forbidden":                  "Запрещено",
		"invalid_request":            "Неверный запрос",
		"phone_already_registered":   "Этот номер телефона уже зарегистрирован",
		"invalid_credentials":        "Неверный номер телефона или пароль",
		"user_not_found":             "Пользователь не найден",
		"token_invalid":              "Токен недействителен",
		"registration_successful":    "Регистрация успешна",
		"login_successful":           "Вход выполнен успешно",
		"food_not_found":             "Блюдо не найдено",
		"food_created":               "Блюдо создано",
		"food_updated":               "Информация о блюде обновлена",
		"food_deleted":               "Блюдо удалено",
		"only_admin_can_manage_food": "Только админ может управлять блюдами",
		"order_created":              "Заказ создан",
		"order_not_found":            "Заказ не найден",
		"order_cancelled":            "Заказ отменен",
		"order_status_updated":       "Статус заказа обновлен",
		"food_not_available":         "Блюдо недоступно",
		"invalid_quantity":           "Количество блюда должно быть больше 0",
		"order_confirmed":            "Ваш заказ подтвержден!",
		"new_order":                  "Новый заказ!",
		"order_id":                   "ID заказа:",
		"customer":                   "Клиент:",
		"phone":                      "Телефон:",
		"time":                       "Время:",
		"order_items":                "Состав заказа:",
		"total_amount":               "Общая сумма:",
		"delivery_address":           "Доставка:",
		"pickup":                     "Самовывоз:",
		"restaurant_table":           "В ресторане:",
		"payment_method":             "Способ оплаты:",
		"preparation_time":           "Время приготовления:",
		"additional_notes":           "Дополнительно:",
		"shashlik":                   "Шашлык",
		"milliy_taomlar":             "Национальные блюда",
		"ichimliklar":                "Напитки",
		"salatlar":                   "Салаты",
		"shirinliklar":               "Десерты",
		"delivery":                   "Доставка",
		"own_withdrawal":             "Самовывоз",
		"at_restaurant":              "В ресторане",
		"cash":                       "Наличные",
		"card":                       "Карта",
		"click":                      "Click",
		"payme":                      "Payme",
		"login_required":             "Войдите в систему для оформления заказа",
		"cart_empty":                 "Корзина пуста",
		"insufficient_stock":         "Недостаточное количество",
		"order_processing":           "Заказ обрабатывается",
		"file_uploaded":              "Файл загружен",
		"invalid_file":               "Неверный файл",
		"file_too_large":             "Файл слишком большой",
		"order_status_pending":       "Ваш заказ принят",
		"order_status_confirmed":     "Ваш заказ подтвержден!",
		"order_status_preparing":     "Ваш заказ готовится",
		"order_status_ready":         "Ваш заказ готов!",
		"order_status_delivered":     "Ваш заказ доставлен",
		"order_status_cancelled":     "Ваш заказ отменен",
		"review_created":             "Отзыв добавлен",
		"review_updated":             "Отзыв обновлен",
		"review_deleted":             "Отзыв удален",
	},
	"en": {
		"success":                    "Success",
		"error":                      "Error",
		"not_found":                  "Not found",
		"unauthorized":               "Unauthorized",
		"forbidden":                  "Forbidden",
		"invalid_request":            "Invalid request",
		"phone_already_registered":   "This phone number is already registered",
		"invalid_credentials":        "Invalid phone number or password",
		"user_not_found":             "User not found",
		"token_invalid":              "Token is invalid",
		"registration_successful":    "Registration successful",
		"login_successful":           "Login successful",
		"food_not_found":             "Food not found",
		"food_created":               "Food created",
		"food_updated":               "Food information updated",
		"food_deleted":               "Food deleted",
		"only_admin_can_manage_food": "Only admin can manage food",
		"order_created":              "Order created",
		"order_not_found":            "Order not found",
		"order_cancelled":            "Order cancelled",
		"order_status_updated":       "Order status updated",
		"food_not_available":         "Food not available",
		"invalid_quantity":           "Food quantity must be greater than 0",
		"order_confirmed":            "Your order has been confirmed!",
		"new_order":                  "New order!",
		"order_id":                   "Order ID:",
		"customer":                   "Customer:",
		"phone":                      "Phone:",
		"time":                       "Time:",
		"order_items":                "Order items:",
		"total_amount":               "Total amount:",
		"delivery_address":           "Delivery:",
		"pickup":                     "Pickup:",
		"restaurant_table":           "At restaurant:",
		"payment_method":             "Payment method:",
		"preparation_time":           "Preparation time:",
		"additional_notes":           "Additional:",
		"shashlik":                   "Barbecue",
		"milliy_taomlar":             "National dishes",
		"ichimliklar":                "Drinks",
		"salatlar":                   "Salads",
		"shirinliklar":               "Desserts",
		"delivery":                   "Delivery",
		"own_withdrawal":             "Pickup",
		"at_restaurant":              "At restaurant",
		"cash":                       "Cash",
		"card":                       "Card",
		"click":                      "Click",
		"payme":                      "Payme",
		"login_required":             "Please login to place an order",
		"cart_empty":                 "Cart is empty",
		"insufficient_stock":         "Insufficient stock",
		"order_processing":           "Order is being processed",
		"file_uploaded":              "File uploaded",
		"invalid_file":               "Invalid file",
		"file_too_large":             "File too large",
		"order_status_pending":       "Your order has been received",
		"order_status_confirmed":     "Your order has been confirmed!",
		"order_status_preparing":     "Your order is being prepared",
		"order_status_ready":         "Your order is ready!",
		"order_status_delivered":     "Your order has been delivered",
		"order_status_cancelled":     "Your order has been cancelled",
		"review_created":             "Review added",
		"review_updated":             "Review updated",
		"review_deleted":             "Review deleted",
	},
}

// Structures
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
	ID              string              `json:"id"`
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
	ID          string `json:"id"`
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
	FoodID    string    `json:"food_id"`
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

type DailyCounter struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
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
}

type CartItem struct {
	FoodID   string `json:"food_id" binding:"required"`
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
	FoodID  string `json:"food_id" binding:"required"`
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"required"`
}

type ReviewUpdate struct {
	Rating  *int    `json:"rating,omitempty"`
	Comment *string `json:"comment,omitempty"`
}

type LanguageRequest struct {
	Language string `json:"language" binding:"required"`
}

// Claims JWT uchun
type Claims struct {
	Number string `json:"sub"`
	Role   string `json:"role"`
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	OrderID string      `json:"order_id,omitempty"`
}

// Restaurant tables
var RestaurantTables = map[string]string{
	"Zal-1 Stol-1": "93e05d01c3304b3b9dc963db187dbb51",
	"Zal-1 Stol-2": "73d6827a734a43b6ad779b5979bb9c6a",
	"Zal-1 Stol-3": "dc6e76e87f9e42a08a4e1198fc5f89a0",
	"Zal-1 Stol-4": "70a53b0ac3264fce88d9a4b7d3a7fa5e",
}

// ========== JSON FILE OPERATIONS ==========

func initDataDirectory() error {
	if _, err := os.Stat(DATA_DIR); os.IsNotExist(err) {
		if err := os.MkdirAll(DATA_DIR, 0755); err != nil {
			return fmt.Errorf("Data papkasini yaratishda xatolik: %v", err)
		}
	}

	// Initialize empty JSON files if they don't exist
	files := []string{USERS_FILE, FOODS_FILE, ORDERS_FILE, REVIEWS_FILE, FILE_UPLOADS_FILE, DAILY_COUNTER_FILE}

	for _, filename := range files {
		filepath := filepath.Join(DATA_DIR, filename)
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			emptyData := "[]"
			if filename == DAILY_COUNTER_FILE {
				emptyData = `{"date": "", "count": 0}`
			}
			if err := os.WriteFile(filepath, []byte(emptyData), 0644); err != nil {
				return fmt.Errorf("%s faylini yaratishda xatolik: %v", filename, err)
			}
		}
	}

	log.Println("✅ JSON ma'lumotlar papkasi muvaffaqiyatli tayyorlandi")
	return nil
}

// Generic JSON file operations
func readJSONFile(filename string, data interface{}) error {
	filepath := filepath.Join(DATA_DIR, filename)
	file, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, data)
}

func writeJSONFile(filename string, data interface{}) error {
	filepath := filepath.Join(DATA_DIR, filename)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, jsonData, 0644)
}

// ========== USER OPERATIONS ==========

func getAllUsers() ([]User, error) {
	usersLock.RLock()
	defer usersLock.RUnlock()

	var users []User
	err := readJSONFile(USERS_FILE, &users)
	return users, err
}

func saveUsers(users []User) error {
	usersLock.Lock()
	defer usersLock.Unlock()

	return writeJSONFile(USERS_FILE, users)
}

func getUserByNumber(number string) (*User, error) {
	users, err := getAllUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Number == number {
			return &user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func getUserByID(userID string) (*User, error) {
	users, err := getAllUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.ID == userID {
			return &user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func createUser(user *User) error {
	users, err := getAllUsers()
	if err != nil {
		return err
	}

	// Check if user already exists
	for _, existingUser := range users {
		if existingUser.Number == user.Number {
			return fmt.Errorf("user already exists")
		}
	}

	users = append(users, *user)
	return saveUsers(users)
}

// ========== FOOD OPERATIONS ==========

func getAllFoods() ([]Food, error) {
	foodsLock.RLock()
	defer foodsLock.RUnlock()

	var foods []Food
	err := readJSONFile(FOODS_FILE, &foods)
	return foods, err
}

func saveFoods(foods []Food) error {
	foodsLock.Lock()
	defer foodsLock.Unlock()

	return writeJSONFile(FOODS_FILE, foods)
}

func getFoodByID(foodID string) (*Food, error) {
	foods, err := getAllFoods()
	if err != nil {
		return nil, err
	}

	for _, food := range foods {
		if food.ID == foodID {
			return &food, nil
		}
	}
	return nil, fmt.Errorf("food not found")
}

func createFood(food *Food) error {
	foods, err := getAllFoods()
	if err != nil {
		return err
	}

	foods = append(foods, *food)
	return saveFoods(foods)
}

func updateFood(updatedFood *Food) error {
	foods, err := getAllFoods()
	if err != nil {
		return err
	}

	for i, food := range foods {
		if food.ID == updatedFood.ID {
			foods[i] = *updatedFood
			return saveFoods(foods)
		}
	}
	return fmt.Errorf("food not found")
}

func deleteFood(foodID string) error {
	foods, err := getAllFoods()
	if err != nil {
		return err
	}

	for i, food := range foods {
		if food.ID == foodID {
			foods = append(foods[:i], foods[i+1:]...)
			return saveFoods(foods)
		}
	}
	return fmt.Errorf("food not found")
}

// ========== ORDER OPERATIONS ==========

func getAllOrders() ([]Order, error) {
	ordersLock.RLock()
	defer ordersLock.RUnlock()

	var orders []Order
	err := readJSONFile(ORDERS_FILE, &orders)
	return orders, err
}

func saveOrders(orders []Order) error {
	ordersLock.Lock()
	defer ordersLock.Unlock()

	return writeJSONFile(ORDERS_FILE, orders)
}

func getOrderByID(orderID string) (*Order, error) {
	orders, err := getAllOrders()
	if err != nil {
		return nil, err
	}

	for _, order := range orders {
		if order.OrderID == orderID {
			return &order, nil
		}
	}
	return nil, fmt.Errorf("order not found")
}

func createOrder(order *Order) error {
	orders, err := getAllOrders()
	if err != nil {
		return err
	}

	orders = append(orders, *order)
	return saveOrders(orders)
}

func updateOrder(updatedOrder *Order) error {
	orders, err := getAllOrders()
	if err != nil {
		return err
	}

	for i, order := range orders {
		if order.OrderID == updatedOrder.OrderID {
			orders[i] = *updatedOrder
			return saveOrders(orders)
		}
	}
	return fmt.Errorf("order not found")
}

// ========== REVIEW OPERATIONS ==========

func getAllReviews() ([]Review, error) {
	reviewsLock.RLock()
	defer reviewsLock.RUnlock()

	var reviews []Review
	err := readJSONFile(REVIEWS_FILE, &reviews)
	return reviews, err
}

func saveReviews(reviews []Review) error {
	reviewsLock.Lock()
	defer reviewsLock.Unlock()

	return writeJSONFile(REVIEWS_FILE, reviews)
}

func getReviewsByFoodID(foodID string) ([]Review, error) {
	reviews, err := getAllReviews()
	if err != nil {
		return nil, err
	}

	var foodReviews []Review
	for _, review := range reviews {
		if review.FoodID == foodID {
			// Get user name
			if user, err := getUserByID(review.UserID); err == nil {
				review.UserName = user.FullName
			}
			foodReviews = append(foodReviews, review)
		}
	}
	return foodReviews, nil
}

func createReview(review *Review) error {
	reviews, err := getAllReviews()
	if err != nil {
		return err
	}

	// Check if user already reviewed this food
	for i, existingReview := range reviews {
		if existingReview.UserID == review.UserID && existingReview.FoodID == review.FoodID {
			// Update existing review
			reviews[i] = *review
			return saveReviews(reviews)
		}
	}

	// Add new review
	reviews = append(reviews, *review)
	return saveReviews(reviews)
}

func updateFoodRating(foodID string) error {
	reviews, err := getAllReviews()
	if err != nil {
		return err
	}

	// Calculate average rating
	var totalRating float64
	var count int
	for _, review := range reviews {
		if review.FoodID == foodID {
			totalRating += float64(review.Rating)
			count++
		}
	}

	if count > 0 {
		averageRating := totalRating / float64(count)

		// Update food rating
		foods, err := getAllFoods()
		if err != nil {
			return err
		}

		for i, food := range foods {
			if food.ID == foodID {
				foods[i].Rating = averageRating
				foods[i].ReviewCount = count
				return saveFoods(foods)
			}
		}
	}

	return nil
}

// ========== FILE UPLOAD OPERATIONS ==========

func getAllFileUploads() ([]FileUpload, error) {
	uploadsLock.RLock()
	defer uploadsLock.RUnlock()

	var uploads []FileUpload
	err := readJSONFile(FILE_UPLOADS_FILE, &uploads)
	return uploads, err
}

func saveFileUploads(uploads []FileUpload) error {
	uploadsLock.Lock()
	defer uploadsLock.Unlock()

	return writeJSONFile(FILE_UPLOADS_FILE, uploads)
}

func createFileUpload(upload *FileUpload) error {
	uploads, err := getAllFileUploads()
	if err != nil {
		return err
	}

	uploads = append(uploads, *upload)
	return saveFileUploads(uploads)
}

func deleteFileUpload(fileID string) error {
	uploads, err := getAllFileUploads()
	if err != nil {
		return err
	}

	for i, upload := range uploads {
		if upload.ID == fileID {
			uploads = append(uploads[:i], uploads[i+1:]...)
			return saveFileUploads(uploads)
		}
	}
	return fmt.Errorf("file not found")
}

func getFileUploadByID(fileID string) (*FileUpload, error) {
	uploads, err := getAllFileUploads()
	if err != nil {
		return nil, err
	}

	for _, upload := range uploads {
		if upload.ID == fileID {
			return &upload, nil
		}
	}
	return nil, fmt.Errorf("file not found")
}

// ========== DAILY COUNTER OPERATIONS ==========

func getDailyCounter() (*DailyCounter, error) {
	counterLock.RLock()
	defer counterLock.RUnlock()

	var counter DailyCounter
	err := readJSONFile(DAILY_COUNTER_FILE, &counter)
	return &counter, err
}

func saveDailyCounter(counter *DailyCounter) error {
	counterLock.Lock()
	defer counterLock.Unlock()

	return writeJSONFile(DAILY_COUNTER_FILE, counter)
}

// ========== UTILITY FUNCTIONS ==========

func getTranslation(key, lang string) string {
	if lang == "" {
		lang = "uz"
	}
	if translations, exists := TRANSLATIONS[lang]; exists {
		if translation, exists := translations[key]; exists {
			return translation
		}
	}
	// Default o'zbek tilida qaytarish
	if translations, exists := TRANSLATIONS["uz"]; exists {
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
	return "uz"
}

func createResponse(messageKey, lang string) gin.H {
	message := getTranslation(messageKey, lang)
	return gin.H{
		"message":  message,
		"language": lang,
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.New().String()[:8])
}

func generateFoodID() string {
	foods, err := getAllFoods()
	if err != nil {
		return "amur_1"
	}

	maxID := 0
	re := regexp.MustCompile(`amur_(\d+)`)

	for _, food := range foods {
		matches := re.FindStringSubmatch(food.ID)
		if len(matches) == 2 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				if num > maxID {
					maxID = num
				}
			}
		}
	}

	return fmt.Sprintf("amur_%d", maxID+1)
}

func generateOrderID() string {
	today := time.Now().Format("2006-01-02")

	counter, err := getDailyCounter()
	if err != nil {
		counter = &DailyCounter{Date: today, Count: 0}
	}

	if counter.Date != today {
		counter.Date = today
		counter.Count = 0
	}

	counter.Count++
	saveDailyCounter(counter)

	return fmt.Sprintf("%s-%d", today, counter.Count)
}

func hashPassword(password string) string {
	// Simple hash - in production use bcrypt
	return fmt.Sprintf("%x", password)
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
	return "Noma'lum stol"
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

func getLocalizedFood(food *Food, lang string) *Food {
	localizedFood := *food

	// Ko'p tilli nomni olish
	if food.Names != nil {
		if name, exists := food.Names[lang]; exists {
			localizedFood.Name = name
		} else if name, exists := food.Names["uz"]; exists {
			localizedFood.Name = name
		}
	}

	// Ko'p tilli tavsifni olish
	if food.Descriptions != nil {
		if desc, exists := food.Descriptions[lang]; exists {
			localizedFood.Description = desc
		} else if desc, exists := food.Descriptions["uz"]; exists {
			localizedFood.Description = desc
		}
	}

	// Kategoriya nomini tarjima qilish
	categoryKey := strings.ToLower(strings.ReplaceAll(food.Category, " ", "_"))
	localizedFood.CategoryName = getTranslation(categoryKey, lang)

	// Chegirma hisoblash
	if food.Discount > 0 {
		localizedFood.OriginalPrice = food.Price
		localizedFood.Price = food.Price - (food.Price * food.Discount / 100)
	}

	return &localizedFood
}

func getAllLocalizedFoods(lang string, isAdmin bool) ([]*Food, error) {
	foods, err := getAllFoods()
	if err != nil {
		return nil, err
	}

	var localizedFoods []*Food
	for _, food := range foods {
		// Non-admin users only see available foods
		if !isAdmin && (!food.IsThere || food.Stock <= 0) {
			continue
		}

		localizedFood := getLocalizedFood(&food, lang)
		localizedFoods = append(localizedFoods, localizedFood)
	}

	return localizedFoods, nil
}

// ========== WEBSOCKET FUNCTIONS ==========

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade xatoligi: %v", err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	log.Printf("WebSocket mijoz ulandi. Jami mijozlar: %d", len(clients))

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket o'qish xatoligi: %v", err)
			delete(clients, conn)
			log.Printf("WebSocket mijoz uzildi. Qolgan mijozlar: %d", len(clients))
			break
		}
	}
}

func broadcastToClients(message WSMessage) {
	jsonData, _ := json.Marshal(message)
	for client := range clients {
		err := client.WriteJSON(message)
		if err != nil {
			log.Printf("WebSocket yozish xatoligi: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
	log.Printf("WebSocket xabar yuborildi: %s", string(jsonData))
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
}

// ========== MIDDLEWARE ==========

func corsMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
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
			lang := getUserLanguage(c.Request.Header)
			c.JSON(http.StatusForbidden, createResponse("forbidden", lang))
			c.Abort()
			return
		}

		c.Next()
	})
}

// ========== API HANDLERS ==========

// Root handler
func rootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":             "Restaurant API - JSON Version",
		"version":             "6.0.0",
		"supported_languages": []string{"uz", "ru", "en"},
		"features": []string{
			"JSON File Storage",
			"File Upload with Food Names",
			"Auto-incremental Food IDs (amur_1, amur_2, etc.)",
			"GPS Coordinates for Delivery",
			"Stock Management",
			"Public Foods API",
			"Real-time Order Tracking",
			"WebSocket Support",
			"Advanced Search",
			"Review System",
			"Multi-language Support",
		},
		"endpoints": gin.H{
			"foods":      "/api/foods (PUBLIC)",
			"categories": "/api/categories (PUBLIC)",
			"search":     "/api/search (PUBLIC)",
			"upload":     "/api/upload (PUBLIC/AUTH)",
			"orders":     "/api/orders (AUTH)",
			"reviews":    "/api/reviews (AUTH)",
			"websocket":  "/ws",
			"statistics": "/api/admin/statistics (ADMIN)",
		},
		"storage": gin.H{
			"type":   "JSON Files",
			"status": "ready",
		},
		"integrations": gin.H{
			"file_upload":  "enabled",
			"gps_tracking": "enabled",
		},
	})
}

// Authentication handlers
func register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lang := req.Language
	if lang == "" {
		lang = "uz"
	}

	// Check if user already exists
	if _, err := getUserByNumber(req.Number); err == nil {
		c.JSON(http.StatusBadRequest, createResponse("phone_already_registered", lang))
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Foydalanuvchi yaratishda xatolik"})
		return
	}

	token, err := createToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token yaratishda xatolik"})
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
		c.JSON(http.StatusUnauthorized, createResponse("invalid_credentials", "uz"))
		return
	}

	if user.Password != hashPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, createResponse("invalid_credentials", user.Language))
		return
	}

	token, err := createToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token yaratishda xatolik"})
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
		lang := getUserLanguage(c.Request.Header)
		c.JSON(http.StatusNotFound, createResponse("user_not_found", lang))
		return
	}

	// Remove password
	userResponse := *userDB
	userResponse.Password = ""

	c.JSON(http.StatusOK, userResponse)
}

// Category handlers
func getCategories(c *gin.Context) {
	lang := getUserLanguage(c.Request.Header)

	categories := []gin.H{
		{"key": "shashlik", "name": getTranslation("shashlik", lang)},
		{"key": "milliy_taomlar", "name": getTranslation("milliy_taomlar", lang)},
		{"key": "ichimliklar", "name": getTranslation("ichimliklar", lang)},
		{"key": "salatlar", "name": getTranslation("salatlar", lang)},
		{"key": "shirinliklar", "name": getTranslation("shirinliklar", lang)},
	}

	c.JSON(http.StatusOK, categories)
}

// Food handlers
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

	// Check if admin
	isAdmin := false
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		isAdmin = (user.Role == "admin")
	}

	foods, err := getAllLocalizedFoods(lang, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	// Apply filters
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

	// Sort
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
	case "id_asc":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].ID < foods[j].ID
		})
	case "id_desc":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].ID > foods[j].ID
		})
	case "name":
		sort.Slice(foods, func(i, j int) bool {
			return foods[i].Name < foods[j].Name
		})
	default:
		// Default: ID bo'yicha saralash
		sort.Slice(foods, func(i, j int) bool {
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

	// Fix image URLs
	hostURL := getHostURL(c)
	for _, food := range foods {
		if food.ImageURL != "" && !strings.HasPrefix(food.ImageURL, "http") {
			food.ImageURL = hostURL + food.ImageURL
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
	foodID := c.Param("food_id")
	lang := getUserLanguage(c.Request.Header)

	food, err := getFoodByID(foodID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("food_not_found", lang))
		return
	}

	localizedFood := getLocalizedFood(food, lang)

	// Fix image URL
	if localizedFood.ImageURL != "" && !strings.HasPrefix(localizedFood.ImageURL, "http") {
		localizedFood.ImageURL = getHostURL(c) + localizedFood.ImageURL
	}

	c.JSON(http.StatusOK, localizedFood)
}

func createFoodHandler(c *gin.Context) {
	var req FoodCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate ID
	foodID := generateFoodID()

	if req.PreparationTime == 0 {
		req.PreparationTime = 15
	}
	if req.Stock == 0 {
		req.Stock = 100
	}

	// Multi-language names
	names := map[string]string{
		"uz": req.NameUz,
		"ru": req.NameRu,
		"en": req.NameEn,
	}

	// Multi-language descriptions
	descriptions := map[string]string{
		"uz": req.DescriptionUz,
		"ru": req.DescriptionRu,
		"en": req.DescriptionEn,
	}

	// Multi-language ingredients
	ingredients := map[string][]string{
		"uz": req.IngredientsUz,
		"ru": req.IngredientsRu,
		"en": req.IngredientsEn,
	}

	// Multi-language allergens
	allergens := map[string][]string{
		"uz": req.AllergensUz,
		"ru": req.AllergensRu,
		"en": req.AllergensEn,
	}

	food := &Food{
		ID:              foodID,
		Names:           names,
		Name:            req.NameUz,
		Descriptions:    descriptions,
		Description:     req.DescriptionUz,
		Category:        req.Category,
		Price:           req.Price,
		IsThere:         req.IsThere,
		ImageURL:        req.ImageURL,
		Ingredients:     ingredients,
		Allergens:       allergens,
		Rating:          0.0,
		ReviewCount:     0,
		PreparationTime: req.PreparationTime,
		Stock:           req.Stock,
		IsPopular:       req.IsPopular,
		Discount:        req.Discount,
		Comment:         req.Comment,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := createFood(food); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ovqat yaratishda xatolik"})
		return
	}

	c.JSON(http.StatusCreated, food)
}

func updateFoodHandler(c *gin.Context) {
	foodID := c.Param("food_id")
	lang := getUserLanguage(c.Request.Header)

	food, err := getFoodByID(foodID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("food_not_found", lang))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	if name, ok := updates["name"].(string); ok {
		food.Name = name
	}
	if category, ok := updates["category"].(string); ok {
		food.Category = category
	}
	if price, ok := updates["price"].(float64); ok {
		food.Price = int(price)
	}
	if description, ok := updates["description"].(string); ok {
		food.Description = description
	}
	if isThere, ok := updates["isThere"].(bool); ok {
		food.IsThere = isThere
	}
	if imageURL, ok := updates["imageUrl"].(string); ok {
		food.ImageURL = imageURL
	}
	if prepTime, ok := updates["preparation_time"].(float64); ok {
		food.PreparationTime = int(prepTime)
	}
	if stock, ok := updates["stock"].(float64); ok {
		food.Stock = int(stock)
	}
	if isPopular, ok := updates["is_popular"].(bool); ok {
		food.IsPopular = isPopular
	}
	if discount, ok := updates["discount"].(float64); ok {
		food.Discount = int(discount)
	}

	food.UpdatedAt = time.Now()

	if err := updateFood(food); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ovqat yangilashda xatolik"})
		return
	}

	c.JSON(http.StatusOK, food)
}

func deleteFoodHandler(c *gin.Context) {
	foodID := c.Param("food_id")
	lang := getUserLanguage(c.Request.Header)

	// Check if food exists
	_, err := getFoodByID(foodID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("food_not_found", lang))
		return
	}

	// Delete food
	if err := deleteFood(foodID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ovqat o'chirishda xatolik"})
		return
	}

	c.JSON(http.StatusOK, createResponse("food_deleted", lang))
}

// Order handlers
func createOrderHandler(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lang := getUserLanguage(c.Request.Header)

	// User verification
	var user *Claims
	if userInterface, exists := c.Get("user"); exists {
		user = userInterface.(*Claims)
	} else {
		c.JSON(http.StatusUnauthorized, createResponse("login_required", lang))
		return
	}

	// Check cart
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, createResponse("cart_empty", lang))
		return
	}

	log.Printf("Creating order for user: %s, items count: %d", user.Number, len(req.Items))

	// Check foods and stock
	var orderedFoods []OrderFood
	totalPrice := 0
	totalPrepTime := 0

	for _, item := range req.Items {
		log.Printf("Processing food_id: %s, quantity: %d", item.FoodID, item.Quantity)

		food, err := getFoodByID(item.FoodID)
		if err != nil {
			log.Printf("Food not found error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   getTranslation("food_not_available", lang),
				"food_id": item.FoodID,
				"details": err.Error(),
			})
			return
		}

		if !food.IsThere || food.Stock <= 0 {
			log.Printf("Food not available: isThere=%v, stock=%d", food.IsThere, food.Stock)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   getTranslation("food_not_available", lang),
				"food_id": item.FoodID,
			})
			return
		}

		// Stock check
		if food.Stock < item.Quantity {
			log.Printf("Insufficient stock: required=%d, available=%d", item.Quantity, food.Stock)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":     getTranslation("insufficient_stock", lang),
				"food_id":   item.FoodID,
				"required":  item.Quantity,
				"available": food.Stock,
			})
			return
		}

		localizedFood := getLocalizedFood(food, lang)
		foodTotalPrice := localizedFood.Price * item.Quantity
		prepTime := food.PreparationTime
		if prepTime > totalPrepTime {
			totalPrepTime = prepTime
		}

		orderedFood := OrderFood{
			ID:          food.ID,
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
			log.Printf("Stock yangilashda xatolik: %v", err)
		}
	}

	log.Printf("Order foods processed successfully, total_price: %d", totalPrice)

	// Delivery info
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

		// Add phone
		if phone, ok := req.DeliveryInfo["phone"].(string); ok {
			deliveryInfo["phone"] = phone
		}

		// Add coordinates if available
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

	// Payment info
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
	userName := "Foydalanuvchi"
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
				Note:      "Buyurtma yaratildi",
			},
		},
		CreatedAt: orderTime,
		UpdatedAt: orderTime,
	}

	log.Printf("Order object created, attempting to save...")

	if err := createOrder(order); err != nil {
		log.Printf("Error creating order: %v", err)

		// Rollback stock
		for _, item := range req.Items {
			if food, err := getFoodByID(item.FoodID); err == nil {
				food.Stock += item.Quantity
				updateFood(food)
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Buyurtma yaratishda xatolik",
			"details": err.Error(),
		})
		return
	}

	log.Printf("Order created successfully: %s", orderID)

	// Real-time updates
	go func() {
		sendNewOrderNotification(order)
		sendOrderUpdate(orderID, OrderPending, getTranslation("order_created", lang))
	}()

	c.JSON(http.StatusCreated, gin.H{
		"order":          order,
		"message":        getTranslation("order_created", lang),
		"estimated_time": totalPrepTime,
		"order_tracking": fmt.Sprintf("/api/orders/%s/track", orderID),
	})
}

func getOrdersHandler(c *gin.Context) {
	user := c.MustGet("user").(*Claims)
	getUserLanguage(c.Request.Header)
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	orders, err := getAllOrders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	// Filter orders
	var filteredOrders []Order
	for _, order := range orders {
		// Admin can see all orders, users only their own
		if user.Role != "admin" && order.UserNumber != user.Number {
			continue
		}

		// Status filter
		if status != "" && string(order.Status) != status {
			continue
		}

		filteredOrders = append(filteredOrders, order)
	}

	// Sort by order time (newest first)
	sort.Slice(filteredOrders, func(i, j int) bool {
		return filteredOrders[i].OrderTime.After(filteredOrders[j].OrderTime)
	})

	// Pagination
	total := len(filteredOrders)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		filteredOrders = []Order{}
	} else {
		if end > total {
			end = total
		}
		filteredOrders = filteredOrders[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": filteredOrders,
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
	lang := getUserLanguage(c.Request.Header)

	order, err := getOrderByID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("order_not_found", lang))
		return
	}

	// Users can only see their own orders
	if user.Role != "admin" && order.UserNumber != user.Number {
		c.JSON(http.StatusForbidden, createResponse("forbidden", lang))
		return
	}

	c.JSON(http.StatusOK, order)
}

func trackOrderHandler(c *gin.Context) {
	orderID := c.Param("order_id")
	lang := getUserLanguage(c.Request.Header)

	order, err := getOrderByID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("order_not_found", lang))
		return
	}

	// Tracking info
	trackingInfo := gin.H{
		"order_id":       order.OrderID,
		"status":         order.Status,
		"estimated_time": order.EstimatedTime,
		"order_time":     order.OrderTime,
		"status_history": order.StatusHistory,
	}

	// Calculate time
	if order.EstimatedTime != nil {
		elapsed := int(time.Since(order.OrderTime).Minutes())
		remaining := *order.EstimatedTime - elapsed
		if remaining < 0 {
			remaining = 0
		}
		trackingInfo["remaining_time"] = remaining
		trackingInfo["elapsed_time"] = elapsed
	}

	c.JSON(http.StatusOK, trackingInfo)
}

func updateOrderStatusHandler(c *gin.Context) {
	orderID := c.Param("order_id")
	lang := getUserLanguage(c.Request.Header)

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
		c.JSON(http.StatusNotFound, createResponse("order_not_found", lang))
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buyurtma yangilashda xatolik"})
		return
	}

	// Real-time update
	message := getTranslation("order_status_updated", lang)
	sendOrderUpdate(orderID, req.Status, message)

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"order":   order,
	})
}

func cancelOrderHandler(c *gin.Context) {
	orderID := c.Param("order_id")
	user := c.MustGet("user").(*Claims)
	lang := getUserLanguage(c.Request.Header)

	order, err := getOrderByID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("order_not_found", lang))
		return
	}

	// Only order owner or admin can cancel
	if order.UserNumber != user.Number && user.Role != "admin" {
		c.JSON(http.StatusForbidden, createResponse("forbidden", lang))
		return
	}

	// Can only cancel pending or confirmed orders
	if order.Status != OrderPending && order.Status != OrderConfirmed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Buyurtmani bekor qilib bo'lmaydi",
		})
		return
	}

	// Return stock
	for _, orderFood := range order.Foods {
		if food, err := getFoodByID(orderFood.ID); err == nil {
			food.Stock += orderFood.Count
			updateFood(food)
		}
	}

	// Add to status history
	statusUpdate := StatusUpdate{
		Status:    OrderCancelled,
		Timestamp: time.Now(),
		Note:      "Buyurtma bekor qilindi",
	}
	order.StatusHistory = append(order.StatusHistory, statusUpdate)
	order.Status = OrderCancelled

	if err := updateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buyurtma yangilashda xatolik"})
		return
	}

	// Real-time update
	sendOrderUpdate(orderID, OrderCancelled, getTranslation("order_cancelled", lang))

	c.JSON(http.StatusOK, createResponse("order_cancelled", lang))
}

// Review handlers
func createReviewHandler(c *gin.Context) {
	var req ReviewCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := c.MustGet("user").(*Claims)
	lang := getUserLanguage(c.Request.Header)

	// Check if food exists
	if _, err := getFoodByID(req.FoodID); err != nil {
		c.JSON(http.StatusNotFound, createResponse("food_not_found", lang))
		return
	}

	review := &Review{
		ID:        generateID("review"),
		UserID:    user.UserID,
		FoodID:    req.FoodID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := createReview(review); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Sharh yaratishda xatolik"})
		return
	}

	// Update food rating
	if err := updateFoodRating(req.FoodID); err != nil {
		log.Printf("Ovqat reytingini yangilashda xatolik: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": getTranslation("review_created", lang),
		"review":  review,
	})
}

func getFoodReviewsHandler(c *gin.Context) {
	foodID := c.Param("food_id")
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	reviews, err := getReviewsByFoodID(foodID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	// Sort by creation time (newest first)
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})

	// Pagination
	total := len(reviews)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		reviews = []Review{}
	} else {
		if end > total {
			end = total
		}
		reviews = reviews[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

// Search handler
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

	// Check if admin
	isAdmin := false
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		isAdmin = (user.Role == "admin")
	}

	foods, err := getAllLocalizedFoods(lang, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
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
			} else if ingredientsList, ok := food.Ingredients["uz"]; ok {
				for _, ingredient := range ingredientsList {
					if strings.Contains(strings.ToLower(ingredient), searchLower) {
						results = append(results, food)
						break
					}
				}
			}
		}
	}

	// Apply filters
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

	// Fix image URLs
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

// File upload handlers
func uploadFile(c *gin.Context) {
	lang := getUserLanguage(c.Request.Header)

	// User verification (optional)
	var uploaderID string
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*Claims)
		uploaderID = user.UserID
	}

	// Get file
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fayl tanlanmadi"})
		return
	}
	defer file.Close()

	// Check file size
	if fileHeader.Size > MAX_FILE_SIZE {
		c.JSON(http.StatusBadRequest, createResponse("file_too_large", lang))
		return
	}

	// Check file type
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		c.JSON(http.StatusBadRequest, createResponse("invalid_file", lang))
		return
	}

	// Create upload directory
	if _, err := os.Stat(UPLOAD_DIR); os.IsNotExist(err) {
		os.MkdirAll(UPLOAD_DIR, 0755)
	}

	// Create file name
	originalName := strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
	cleanedName := cleanFileName(originalName)
	ext := filepath.Ext(fileHeader.Filename)

	// Create unique file name
	fileName := fmt.Sprintf("%s_%d%s", cleanedName, time.Now().Unix(), ext)
	filePath := filepath.Join(UPLOAD_DIR, fileName)

	// Save file
	dst, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Faylni saqlashda xatolik"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Faylni nusxalashda xatolik"})
		return
	}

	// Create URL
	fileURL := fmt.Sprintf("%s/uploads/%s", getHostURL(c), fileName)

	// Save to database
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

	if err := createFileUpload(fileUpload); err != nil {
		log.Printf("Fayl ma'lumotlarini saqlashda xatolik: %v", err)
		// Delete file
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotni saqlashda xatolik"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    getTranslation("file_uploaded", lang),
		"file":       fileUpload,
		"url":        fileURL,
		"public_url": fileURL,
	})
}

func getUploadedFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	fileType := c.Query("type") // image, document

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	uploads, err := getAllFileUploads()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	// Filter by type
	var filteredUploads []FileUpload
	for _, upload := range uploads {
		if fileType == "image" && !strings.HasPrefix(upload.MimeType, "image/") {
			continue
		}
		filteredUploads = append(filteredUploads, upload)
	}

	// Sort by creation time (newest first)
	sort.Slice(filteredUploads, func(i, j int) bool {
		return filteredUploads[i].CreatedAt.After(filteredUploads[j].CreatedAt)
	})

	// Pagination
	total := len(filteredUploads)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		filteredUploads = []FileUpload{}
	} else {
		if end > total {
			end = total
		}
		filteredUploads = filteredUploads[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"files": filteredUploads,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + limit - 1) / limit,
		},
	})
}

func deleteFile(c *gin.Context) {
	fileID := c.Param("file_id")
	lang := getUserLanguage(c.Request.Header)

	// Get file from database
	file, err := getFileUploadByID(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, createResponse("not_found", lang))
		return
	}

	// Delete file from disk
	if err := os.Remove(file.FilePath); err != nil {
		log.Printf("Faylni o'chirishda xatolik: %v", err)
	}

	// Delete from database
	if err := deleteFileUpload(fileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotni o'chirishda xatolik"})
		return
	}

	c.JSON(http.StatusOK, createResponse("success", lang))
}

// Statistics handler
func getStatisticsHandler(c *gin.Context) {
	orders, err := getAllOrders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	foods, err := getAllFoods()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	users, err := getAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ma'lumotlarni olishda xatolik"})
		return
	}

	// Calculate statistics
	totalOrders := len(orders)
	var pendingOrders, completedOrders, cancelledOrders, totalRevenue int
	var todayOrders, todayRevenue int

	today := time.Now().Format("2006-01-02")

	for _, order := range orders {
		switch order.Status {
		case OrderPending, OrderConfirmed, OrderPreparing:
			pendingOrders++
		case OrderDelivered:
			completedOrders++
			totalRevenue += order.TotalPrice
		case OrderCancelled:
			cancelledOrders++
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
	var popularFoods int
	for _, food := range foods {
		if food.IsPopular || food.Rating >= 4.0 {
			popularFoods++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_orders":     totalOrders,
		"pending_orders":   pendingOrders,
		"completed_orders": completedOrders,
		"cancelled_orders": cancelledOrders,
		"total_revenue":    totalRevenue,
		"today_orders":     todayOrders,
		"today_revenue":    todayRevenue,
		"total_foods":      totalFoods,
		"total_users":      len(users),
		"popular_foods":    popularFoods,
	})
}

// Initialize test data
func initializeTestData() error {
	// Test admin user
	adminUser := &User{
		ID:        generateID("user"),
		Number:    "770451117",
		Password:  hashPassword("samandar"),
		Role:      "admin",
		FullName:  "Samandar Admin",
		Email:     stringPtr("admin@restaurant.uz"),
		CreatedAt: time.Now(),
		IsActive:  true,
		TgID:      int64Ptr(1713329317),
		Language:  "uz",
	}

	// Check if admin exists
	if _, err := getUserByNumber(adminUser.Number); err != nil {
		if err := createUser(adminUser); err != nil {
			log.Printf("Admin foydalanuvchi yaratishda xatolik: %v", err)
		} else {
			log.Println("✅ Admin foydalanuvchi yaratildi")
		}
	} else {
		log.Println("✅ Admin foydalanuvchi mavjud")
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

	if _, err := getUserByNumber(testUser.Number); err != nil {
		if err := createUser(testUser); err != nil {
			log.Printf("Test foydalanuvchi yaratishda xatolik: %v", err)
		} else {
			log.Println("✅ Test foydalanuvchi yaratildi")
		}
	} else {
		log.Println("✅ Test foydalanuvchi mavjud")
	}

	return nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

// Setup routes
func setupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Middleware
	r.Use(corsMiddleware())

	// Static files
	if _, err := os.Stat(UPLOAD_DIR); os.IsNotExist(err) {
		os.MkdirAll(UPLOAD_DIR, 0755)
	}

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
		// Categories
		public.GET("/categories", getCategories)

		// Foods (public - only available foods)
		public.GET("/foods", optionalAuthMiddleware(), getAllFoodsHandler)
		public.GET("/foods/:food_id", getFoodHandler)

		// Search
		public.GET("/search", optionalAuthMiddleware(), searchHandler)

		// Order tracking (public)
		public.GET("/orders/:order_id/track", trackOrderHandler)

		// Reviews (public read)
		public.GET("/foods/:food_id/reviews", getFoodReviewsHandler)

		// File uploads (public but can be authenticated)
		public.POST("/upload", optionalAuthMiddleware(), uploadFile)
		public.GET("/files", optionalAuthMiddleware(), getUploadedFiles)
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
		// Profile
		protected.GET("/profile", getProfile)

		// Orders
		protected.POST("/orders", createOrderHandler)
		protected.GET("/orders", getOrdersHandler)
		protected.GET("/orders/:order_id", getOrderHandler)
		protected.DELETE("/orders/:order_id", cancelOrderHandler)

		// Reviews
		protected.POST("/reviews", createReviewHandler)

		// File management
		protected.DELETE("/files/:file_id", deleteFile)
	}

	// Admin endpoints
	admin := protected.Group("/admin")
	admin.Use(adminMiddleware())
	{
		// Food management
		admin.POST("/foods", createFoodHandler)
		admin.PUT("/foods/:food_id", updateFoodHandler)
		admin.DELETE("/foods/:food_id", deleteFoodHandler)

		// Order management
		admin.PUT("/orders/:order_id/status", updateOrderStatusHandler)

		// Statistics
		admin.GET("/statistics", getStatisticsHandler)
	}

	return r
}

// Main function
func main() {
	// Initialize data directory
	if err := initDataDirectory(); err != nil {
		log.Fatalf("Data papkasini tayyorlashda xatolik: %v", err)
	}

	// Initialize test data
	if err := initializeTestData(); err != nil {
		log.Printf("Test ma'lumotlarini yaratishda xatolik: %v", err)
	}

	// WebSocket handler
	go func() {
		for {
			select {
			case message := <-broadcast:
				for client := range clients {
					err := client.WriteMessage(websocket.TextMessage, message)
					if err != nil {
						log.Printf("WebSocket xabar yuborishda xatolik: %v", err)
						client.Close()
						delete(clients, client)
					}
				}
			}
		}
	}()

	// Setup routes
	r := setupRoutes()

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Server JSON versiyasida %s portida ishlamoqda", port)
	log.Printf("📂 Ma'lumotlar JSON fayllarida saqlanadi: %s/", DATA_DIR)
	log.Printf("🌐 API dokumentatsiyasi: http://localhost:%s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Serverni ishga tushirishda xatolik: %v", err)
	}
}
