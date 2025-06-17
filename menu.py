from fastapi import FastAPI, HTTPException, Depends, status, File, UploadFile, Form, BackgroundTasks, Request
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, EmailStr, field_validator
from typing import List, Optional, Dict, Any
import jwt
import hashlib
from datetime import datetime, timedelta
import uuid
import os
import shutil
import json
from enum import Enum
import asyncio
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
import aiohttp
import logging

# Logging sozlash
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# KO'P TILLI QO'LLAB-QUVVATLASH
TRANSLATIONS = {
    "uz": {
        # Umumiy
        "success": "Muvaffaqiyatli",
        "error": "Xatolik",
        "not_found": "Topilmadi",
        "unauthorized": "Ruxsat etilmagan",
        "forbidden": "Taqiqlangan",
        "invalid_request": "Noto'g'ri so'rov",
        
        # Autentifikatsiya
        "phone_already_registered": "Bu telefon raqami allaqachon ro'yxatdan o'tgan",
        "invalid_credentials": "Telefon raqami yoki parol noto'g'ri",
        "user_not_found": "Foydalanuvchi topilmadi",
        "token_invalid": "Token yaroqsiz",
        "registration_successful": "Ro'yxatdan o'tish muvaffaqiyatli",
        "login_successful": "Tizimga kirish muvaffaqiyatli",
        
        # Ovqatlar
        "food_not_found": "Ovqat topilmadi",
        "food_created": "Ovqat yaratildi",
        "food_updated": "Ovqat ma'lumotlari yangilandi",
        "food_deleted": "Ovqat o'chirildi",
        "only_admin_can_manage_food": "Faqat admin ovqat boshqara oladi",
        "image_uploaded": "Rasm muvaffaqiyatli yuklandi",
        "invalid_file_format": "Fayl formati qo'llab-quvvatlanmaydi",
        
        # Buyurtmalar
        "order_created": "Buyurtma yaratildi",
        "order_not_found": "Buyurtma topilmadi",
        "order_cancelled": "Buyurtma bekor qilindi",
        "order_status_updated": "Buyurtma holati yangilandi",
        "cannot_cancel_order": "Bu buyurtmani bekor qilib bo'lmaydi",
        "food_not_available": "Ovqat mavjud emas",
        "invalid_quantity": "Ovqat miqdori 0 dan katta bo'lishi kerak",
        "order_confirmed": "Buyurtmangiz tasdiqlandi!",
        "order_preparing": "Buyurtmangiz tayyorlanmoqda...",
        "order_ready": "Buyurtmangiz tayyor!",
        "order_delivered": "Buyurtmangiz yetkazildi!",
        "order_cancelled_msg": "Buyurtmangiz bekor qilindi.",
        
        # Sharhlar
        "review_created": "Sharh qo'shildi",
        "review_deleted": "Sharh o'chirildi",
        "review_not_found": "Sharh topilmadi",
        "already_reviewed": "Siz bu ovqat uchun allaqachon sharh qoldirgan ekansiz",
        "rating_invalid": "Rating 1 dan 5 gacha bo'lishi kerak",
        
        # Bildirishnomalar
        "notification_marked_read": "Bildirishnoma o'qilgan deb belgilandi",
        "all_notifications_read": "ta bildirishnoma o'qilgan deb belgilandi",
        "notification_not_found": "Bildirishnoma topilmadi",
        
        # Aktsiyalar
        "promotion_created": "Aksiya yaratildi",
        "promotion_updated": "Aksiya yangilandi",
        "promotion_deleted": "Aksiya o'chirildi",
        "promotion_not_found": "Aksiya topilmadi",
        "promo_code_invalid": "Promo kod yaroqsiz yoki muddati tugagan",
        
        # Inventar
        "inventory_updated": "Inventar yangilandi",
        "inventory_item_created": "Inventar elementi qo'shildi",
        "inventory_item_deleted": "Inventar elementi o'chirildi",
        "inventory_item_not_found": "Inventar elementi topilmadi",
        "low_stock_warning": "tugab qolmoqda! Qolgan:",
        
        # Xodimlar
        "staff_created": "Xodim qo'shildi",
        "staff_updated": "Xodim ma'lumotlari yangilandi",
        "staff_deleted": "Xodim o'chirildi",
        "staff_not_found": "Xodim topilmadi",
        
        # Telegram xabarlari
        "new_order": "Yangi buyurtma!",
        "order_id": "Buyurtma ID:",
        "customer": "Mijoz:",
        "phone": "Telefon:",
        "time": "Vaqt:",
        "order_items": "Buyurtma tarkibi:",
        "total_amount": "Umumiy summa:",
        "delivery_address": "Yetkazib berish:",
        "pickup": "O'zi olib ketish:",
        "restaurant_table": "Restoranda:",
        "payment_method": "To'lov usuli:",
        "preparation_time": "Tayyorlash vaqti:",
        "additional_notes": "Qo'shimcha:",
        "order_accepted": "Buyurtmangiz qabul qilindi!",
        "order_status_notification": "Buyurtmangiz holati haqida xabar berib turamiz!",
        
        # Kategoriyalar
        "shashlik": "Shashlik",
        "milliy_taomlar": "Milliy taomlar",
        "ichimliklar": "Ichimliklar",
        "salatlar": "Salatlar",
        "shirinliklar": "Shirinliklar",
        
        # Yetkazib berish turlari
        "delivery": "Yetkazib berish",
        "own_withdrawal": "O'zi olib ketish",
        "at_restaurant": "Restoranda",
        
        # To'lov usullari
        "cash": "Naqd",
        "card": "Karta",
        "click": "Click",
        "payme": "Payme",
        
        # Buyurtma holatlari
        "pending": "Kutilmoqda",
        "confirmed": "Tasdiqlangan",
        "preparing": "Tayyorlanmoqda",
        "ready": "Tayyor",
        "delivered": "Yetkazilgan",
        "cancelled": "Bekor qilingan"
    },
    
    "ru": {
        # Общие
        "success": "Успешно",
        "error": "Ошибка",
        "not_found": "Не найдено",
        "unauthorized": "Неавторизован",
        "forbidden": "Запрещено",
        "invalid_request": "Неверный запрос",
        
        # Аутентификация
        "phone_already_registered": "Этот номер телефона уже зарегистрирован",
        "invalid_credentials": "Неверный номер телефона или пароль",
        "user_not_found": "Пользователь не найден",
        "token_invalid": "Токен недействителен",
        "registration_successful": "Регистрация успешна",
        "login_successful": "Вход выполнен успешно",
        
        # Блюда
        "food_not_found": "Блюдо не найдено",
        "food_created": "Блюдо создано",
        "food_updated": "Информация о блюде обновлена",
        "food_deleted": "Блюдо удалено",
        "only_admin_can_manage_food": "Только админ может управлять блюдами",
        "image_uploaded": "Изображение успешно загружено",
        "invalid_file_format": "Формат файла не поддерживается",
        
        # Заказы
        "order_created": "Заказ создан",
        "order_not_found": "Заказ не найден",
        "order_cancelled": "Заказ отменен",
        "order_status_updated": "Статус заказа обновлен",
        "cannot_cancel_order": "Этот заказ нельзя отменить",
        "food_not_available": "Блюдо недоступно",
        "invalid_quantity": "Количество блюда должно быть больше 0",
        "order_confirmed": "Ваш заказ подтвержден!",
        "order_preparing": "Ваш заказ готовится...",
        "order_ready": "Ваш заказ готов!",
        "order_delivered": "Ваш заказ доставлен!",
        "order_cancelled_msg": "Ваш заказ отменен.",
        
        # Отзывы
        "review_created": "Отзыв добавлен",
        "review_deleted": "Отзыв удален",
        "review_not_found": "Отзыв не найден",
        "already_reviewed": "Вы уже оставили отзыв для этого блюда",
        "rating_invalid": "Рейтинг должен быть от 1 до 5",
        
        # Уведомления
        "notification_marked_read": "Уведомление отмечено как прочитанное",
        "all_notifications_read": "уведомлений отмечено как прочитанное",
        "notification_not_found": "Уведомление не найдено",
        
        # Акции
        "promotion_created": "Акция создана",
        "promotion_updated": "Акция обновлена",
        "promotion_deleted": "Акция удалена",
        "promotion_not_found": "Акция не найдена",
        "promo_code_invalid": "Промо-код недействителен или истек",
        
        # Инвентарь
        "inventory_updated": "Инвентарь обновлен",
        "inventory_item_created": "Элемент инвентаря добавлен",
        "inventory_item_deleted": "Элемент инвентаря удален",
        "inventory_item_not_found": "Элемент инвентаря не найден",
        "low_stock_warning": "заканчивается! Осталось:",
        
        # Сотрудники
        "staff_created": "Сотрудник добавлен",
        "staff_updated": "Информация о сотруднике обновлена",
        "staff_deleted": "Сотрудник удален",
        "staff_not_found": "Сотрудник не найден",
        
        # Telegram сообщения
        "new_order": "Новый заказ!",
        "order_id": "ID заказа:",
        "customer": "Клиент:",
        "phone": "Телефон:",
        "time": "Время:",
        "order_items": "Состав заказа:",
        "total_amount": "Общая сумма:",
        "delivery_address": "Доставка:",
        "pickup": "Самовывоз:",
        "restaurant_table": "В ресторане:",
        "payment_method": "Способ оплаты:",
        "preparation_time": "Время приготовления:",
        "additional_notes": "Дополнительно:",
        "order_accepted": "Ваш заказ принят!",
        "order_status_notification": "Мы сообщим вам о статусе заказа!",
        
        # Категории
        "shashlik": "Шашлык",
        "milliy_taomlar": "Национальные блюда",
        "ichimliklar": "Напитки",
        "salatlar": "Салаты",
        "shirinliklar": "Десерты",
        
        # Типы доставки
        "delivery": "Доставка",
        "own_withdrawal": "Самовывоз",
        "at_restaurant": "В ресторане",
        
        # Способы оплаты
        "cash": "Наличные",
        "card": "Карта",
        "click": "Click",
        "payme": "Payme",
        
        # Статусы заказа
        "pending": "Ожидание",
        "confirmed": "Подтвержден",
        "preparing": "Готовится",
        "ready": "Готов",
        "delivered": "Доставлен",
        "cancelled": "Отменен"
    },
    
    "en": {
        # General
        "success": "Success",
        "error": "Error",
        "not_found": "Not found",
        "unauthorized": "Unauthorized",
        "forbidden": "Forbidden",
        "invalid_request": "Invalid request",
        
        # Authentication
        "phone_already_registered": "This phone number is already registered",
        "invalid_credentials": "Invalid phone number or password",
        "user_not_found": "User not found",
        "token_invalid": "Token is invalid",
        "registration_successful": "Registration successful",
        "login_successful": "Login successful",
        
        # Food
        "food_not_found": "Food not found",
        "food_created": "Food created",
        "food_updated": "Food information updated",
        "food_deleted": "Food deleted",
        "only_admin_can_manage_food": "Only admin can manage food",
        "image_uploaded": "Image uploaded successfully",
        "invalid_file_format": "File format not supported",
        
        # Orders
        "order_created": "Order created",
        "order_not_found": "Order not found",
        "order_cancelled": "Order cancelled",
        "order_status_updated": "Order status updated",
        "cannot_cancel_order": "This order cannot be cancelled",
        "food_not_available": "Food not available",
        "invalid_quantity": "Food quantity must be greater than 0",
        "order_confirmed": "Your order has been confirmed!",
        "order_preparing": "Your order is being prepared...",
        "order_ready": "Your order is ready!",
        "order_delivered": "Your order has been delivered!",
        "order_cancelled_msg": "Your order has been cancelled.",
        
        # Reviews
        "review_created": "Review added",
        "review_deleted": "Review deleted",
        "review_not_found": "Review not found",
        "already_reviewed": "You have already reviewed this food",
        "rating_invalid": "Rating must be between 1 and 5",
        
        # Notifications
        "notification_marked_read": "Notification marked as read",
        "all_notifications_read": "notifications marked as read",
        "notification_not_found": "Notification not found",
        
        # Promotions
        "promotion_created": "Promotion created",
        "promotion_updated": "Promotion updated",
        "promotion_deleted": "Promotion deleted",
        "promotion_not_found": "Promotion not found",
        "promo_code_invalid": "Promo code is invalid or expired",
        
        # Inventory
        "inventory_updated": "Inventory updated",
        "inventory_item_created": "Inventory item added",
        "inventory_item_deleted": "Inventory item deleted",
        "inventory_item_not_found": "Inventory item not found",
        "low_stock_warning": "is running low! Remaining:",
        
        # Staff
        "staff_created": "Staff member added",
        "staff_updated": "Staff information updated",
        "staff_deleted": "Staff member deleted",
        "staff_not_found": "Staff member not found",
        
        # Telegram messages
        "new_order": "New order!",
        "order_id": "Order ID:",
        "customer": "Customer:",
        "phone": "Phone:",
        "time": "Time:",
        "order_items": "Order items:",
        "total_amount": "Total amount:",
        "delivery_address": "Delivery:",
        "pickup": "Pickup:",
        "restaurant_table": "At restaurant:",
        "payment_method": "Payment method:",
        "preparation_time": "Preparation time:",
        "additional_notes": "Additional:",
        "order_accepted": "Your order has been accepted!",
        "order_status_notification": "We will notify you about your order status!",
        
        # Categories
        "shashlik": "Barbecue",
        "milliy_taomlar": "National dishes",
        "ichimliklar": "Drinks",
        "salatlar": "Salads",
        "shirinliklar": "Desserts",
        
        # Delivery types
        "delivery": "Delivery",
        "own_withdrawal": "Pickup",
        "at_restaurant": "At restaurant",
        
        # Payment methods
        "cash": "Cash",
        "card": "Card",
        "click": "Click",
        "payme": "Payme",
        
        # Order statuses
        "pending": "Pending",
        "confirmed": "Confirmed",
        "preparing": "Preparing",
        "ready": "Ready",
        "delivered": "Delivered",
        "cancelled": "Cancelled"
    }
}

# Ko'p tilli yordamchi funksiyalar
def get_translation(key: str, lang: str = "uz") -> str:
    """Tarjima olish funksiyasi"""
    if lang not in TRANSLATIONS:
        lang = "uz"  # default til
    return TRANSLATIONS[lang].get(key, key)

def get_user_language(request_headers: dict) -> str:
    """Foydalanuvchi tilini aniqlash"""
    accept_language = request_headers.get("accept-language", "uz")
    
    if "," in accept_language:
        lang = accept_language.split(",")[0]
    else:
        lang = accept_language
    
    if "-" in lang:
        lang = lang.split("-")[0]
    
    supported_languages = ["uz", "ru", "en"]
    if lang.lower() not in supported_languages:
        lang = "uz"
    
    return lang.lower()

def create_response(message_key: str, lang: str = "uz", **kwargs):
    """Ko'p tilli response yaratish"""
    message = get_translation(message_key, lang)
    
    if kwargs:
        try:
            message = message.format(**kwargs)
        except:
            pass
    
    return {"message": message, "language": lang}

# Foydalanuvchi tilini saqlash
USER_LANGUAGES = {}

def set_user_language(user_id: str, language: str):
    """Foydalanuvchi tilini saqlash"""
    if language in ["uz", "ru", "en"]:
        USER_LANGUAGES[user_id] = language
        return True
    return False

def get_user_language_preference(user_id: str) -> str:
    """Foydalanuvchi til sozlamalarini olish"""
    return USER_LANGUAGES.get(user_id, "uz")

# FastAPI ilovasini yaratamiz
app = FastAPI(
    title="Restaurant API",
    description="Restoran uchun ovqatlar boshqaruvi API (Ko'p tilli)",
    version="2.1.0"
)

# Static fayllar uchun papka yaratish
UPLOAD_DIR = "uploads"
if not os.path.exists(UPLOAD_DIR):
    os.makedirs(UPLOAD_DIR)

# Static fayllarni serve qilish
app.mount("/uploads", StaticFiles(directory="uploads"), name="uploads")

# CORS sozlamalari
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# JWT sozlamalari
SECRET_KEY = "restaurant_secret_key_2024"
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 1440  # 24 soat

# Telegram Bot sozlamalari
TELEGRAM_BOT_TOKEN = "7609705273:AAGfEPZ2GYmd8ICgVjXXHGlwXiZWD3nYhP8"  
TELEGRAM_API_URL = f"https://api.telegram.org/bot{TELEGRAM_BOT_TOKEN}/sendMessage"
TELEGRAM_GROUP_ID = -1002783983140  # Telegram guruh ID

# Security
security = HTTPBearer()

# Enums
class OrderStatus(str, Enum):
    PENDING = "pending"
    CONFIRMED = "confirmed"
    PREPARING = "preparing"
    READY = "ready"
    DELIVERED = "delivered"
    CANCELLED = "cancelled"

class DeliveryType(str, Enum):
    DELIVERY = "delivery"
    OWN_WITHDRAWAL = "own_withdrawal"
    AT_RESTAURANT = "atTheRestaurant"

class PaymentStatus(str, Enum):
    PENDING = "pending"
    PAID = "paid"
    FAILED = "failed"
    REFUNDED = "refunded"

class PaymentMethod(str, Enum):
    CASH = "cash"
    CARD = "card"
    CLICK = "click"
    PAYME = "payme"

# Pydantic modellari
class LoginRequest(BaseModel):
    number: str
    password: str

class RegisterRequest(BaseModel):
    number: str
    password: str
    full_name: str
    email: Optional[EmailStr] = None
    tg_id: Optional[int] = None
    language: Optional[str] = "uz"  # Ko'p tilli qo'llab-quvvatlash

class LoginResponse(BaseModel):
    token: str
    role: str
    user_id: str
    language: Optional[str] = "uz"

class User(BaseModel):
    id: str
    number: str
    full_name: str
    email: Optional[str] = None
    role: str
    created_at: str
    is_active: bool = True
    tg_id: Optional[int] = None
    language: Optional[str] = "uz"

class Food(BaseModel):
    id: str
    name: str
    category: str
    price: int
    description: str
    isThere: bool
    imageUrl: str
    ingredients: Optional[List[str]] = []
    allergens: Optional[List[str]] = []
    rating: Optional[float] = 0.0
    review_count: Optional[int] = 0
    preparation_time: Optional[int] = 15
    category_name: Optional[str] = None  # Ko'p tilli kategory nomi

class FoodCreate(BaseModel):
    name: str
    category: str
    price: int
    description: str
    isThere: bool = True
    imageUrl: str
    ingredients: Optional[List[str]] = []
    allergens: Optional[List[str]] = []
    preparation_time: Optional[int] = 15

class FoodUpdate(BaseModel):
    name: Optional[str] = None
    category: Optional[str] = None
    price: Optional[int] = None
    description: Optional[str] = None
    isThere: Optional[bool] = None
    imageUrl: Optional[str] = None
    ingredients: Optional[List[str]] = None
    allergens: Optional[List[str]] = None
    preparation_time: Optional[int] = None

class Review(BaseModel):
    id: str
    user_id: str
    food_id: str
    rating: int
    comment: str
    created_at: str

class ReviewCreate(BaseModel):
    food_id: str
    rating: int
    comment: str
    
    @field_validator('rating')
    @classmethod
    def rating_must_be_valid(cls, v):
        if v < 1 or v > 5:
            raise ValueError('Rating 1 dan 5 gacha bo\'lishi kerak')
        return v

class PaymentInfo(BaseModel):
    method: PaymentMethod
    status: PaymentStatus = PaymentStatus.PENDING
    amount: int
    transaction_id: Optional[str] = None
    payment_time: Optional[str] = None

class OrderRequest(BaseModel):
    food_ids: List[dict]
    to_give: dict
    payment_method: PaymentMethod = PaymentMethod.CASH
    special_instructions: Optional[str] = None

class OrderFood(BaseModel):
    id: str
    name: str
    category: str
    price: int
    description: str
    imageUrl: str
    count: int
    total_price: int

class Order(BaseModel):
    order_id: str
    user_number: str
    user_name: str
    foods: List[OrderFood]
    total_price: int
    order_time: str
    delivery_type: str
    delivery_info: dict
    status: OrderStatus = OrderStatus.PENDING
    payment_info: PaymentInfo
    special_instructions: Optional[str] = None
    estimated_time: Optional[int] = None
    delivered_at: Optional[str] = None

class Notification(BaseModel):
    id: str
    user_id: str
    title: str
    message: str
    is_read: bool = False
    created_at: str
    type: str

class Promotion(BaseModel):
    id: str
    title: str
    description: str
    discount_percent: int
    min_order_amount: int
    start_date: str
    end_date: str
    is_active: bool = True
    promo_code: Optional[str] = None

class Analytics(BaseModel):
    total_orders: int
    total_revenue: int
    popular_foods: List[dict]
    daily_orders: List[dict]
    user_statistics: dict

class InventoryItem(BaseModel):
    id: str
    name: str
    quantity: int
    unit: str
    min_threshold: int
    supplier: Optional[str] = None
    last_updated: str

class InventoryUpdate(BaseModel):
    quantity: int
    operation: str

class Staff(BaseModel):
    id: str
    full_name: str
    position: str
    phone: str
    email: Optional[str] = None
    hire_date: str
    salary: int
    is_active: bool = True

class StaffCreate(BaseModel):
    full_name: str
    position: str
    phone: str
    email: Optional[str] = None
    salary: int

class SupportTicket(BaseModel):
    id: str
    user_id: str
    subject: str
    message: str
    status: str = "open"
    created_at: str
    resolved_at: Optional[str] = None

class TicketCreate(BaseModel):
    subject: str
    message: str

class RestaurantSettings(BaseModel):
    name: str
    address: str
    phone: str
    email: str
    working_hours: dict
    delivery_fee: int
    min_order_amount: int
    max_delivery_distance: int

class LanguageRequest(BaseModel):
    language: str

# Ko'p tilli ovqatlar ma'lumotlari bazasi (Kengaytirilgan)
MULTILINGUAL_FOODS_DB = {
    "amur_1": {
        "id": "amur_1",
        "names": {
            "uz": "Moloti",
            "ru": "Мясной шашлык",
            "en": "Beef Barbecue"
        },
        "descriptions": {
            "uz": "Mol go'shtidan shashlik juda ham mazzali qiyma",
            "ru": "Очень вкусный шашлык из говядины",
            "en": "Very delicious beef barbecue"
        },
        "category": "shashlik",
        "price": 23000,
        "isThere": True,
        "imageUrl": "http://0.0.0.0:8000/uploads/moloti.png",
        "ingredients": {
            "uz": ["Mol go'shti", "Piyoz", "Ziravorlar"],
            "ru": ["Говядина", "Лук", "Специи"],
            "en": ["Beef", "Onion", "Spices"]
        },
        "allergens": [],
        "rating": 4.5,
        "review_count": 15,
        "preparation_time": 20
    },
    "amur_2": {
        "id": "amur_2",
        "names": {
            "uz": "Tovuq Shashlik",
            "ru": "Куриный шашлык",
            "en": "Chicken Barbecue"
        },
        "descriptions": {
            "uz": "Yumshoq tovuq go'shtidan tayyorlangan shashlik",
            "ru": "Шашлык из нежного куриного мяса",
            "en": "Barbecue made from tender chicken meat"
        },
        "category": "shashlik",
        "price": 18000,
        "isThere": True,
        "imageUrl": "https://firebasestorage.googleapis.com/v0/b/amur-restoran.firebasestorage.app/o/olivye.png?alt=media&token=4463f00e-089d-48e3-ac97-96f3ff05a8b6",
        "ingredients": {
            "uz": ["Tovuq go'shti", "Piyoz", "Ziravorlar"],
            "ru": ["Курица", "Лук", "Специи"],
            "en": ["Chicken", "Onion", "Spices"]
        },
        "allergens": [],
        "rating": 4.2,
        "review_count": 12,
        "preparation_time": 15
    },
    "amur_3": {
        "id": "amur_3",
        "names": {
            "uz": "Osh",
            "ru": "Плов",
            "en": "Pilaf"
        },
        "descriptions": {
            "uz": "An'anaviy o'zbek oshi, mol go'shti bilan",
            "ru": "Традиционный узбекский плов с говядиной",
            "en": "Traditional Uzbek pilaf with beef"
        },
        "category": "milliy_taomlar",
        "price": 15000,
        "isThere": True,
        "imageUrl": "https://firebasestorage.googleapis.com/v0/b/amur-restoran.firebasestorage.app/o/ovishi.jpg?alt=media&token=c620f5d3-db4e-47fc-97b3-5a319793132e",
        "ingredients": {
            "uz": ["Guruch", "Mol go'shti", "Sabzi", "Piyoz"],
            "ru": ["Рис", "Говядина", "Морковь", "Лук"],
            "en": ["Rice", "Beef", "Carrot", "Onion"]
        },
        "allergens": [],
        "rating": 4.8,
        "review_count": 25,
        "preparation_time": 30
    },
    "amur_4": {
        "id": "amur_4",
        "names": {
            "uz": "Lag'mon",
            "ru": "Лагман",
            "en": "Lagman"
        },
        "descriptions": {
            "uz": "An'anaviy o'zbek lag'moni, sabzavat bilan",
            "ru": "Традиционный узбекский лагман с овощами",
            "en": "Traditional Uzbek lagman with vegetables"
        },
        "category": "milliy_taomlar",
        "price": 12000,
        "isThere": True,
        "imageUrl": "https://example.com/lagmon.jpg",
        "ingredients": {
            "uz": ["Makaron", "Mol go'shti", "Sabzavotlar"],
            "ru": ["Лапша", "Говядина", "Овощи"],
            "en": ["Noodles", "Beef", "Vegetables"]
        },
        "allergens": {
            "uz": ["Gluten"],
            "ru": ["Глютен"],
            "en": ["Gluten"]
        },
        "rating": 4.3,
        "review_count": 18,
        "preparation_time": 25
    },
    # Qolgan ovqatlarni ham ko'p tilli qilish
    "amur_5": {
        "id": "amur_5",
        "names": {
            "uz": "Moloti 2",
            "ru": "Мясной шашлык 2",
            "en": "Beef Barbecue 2"
        },
        "descriptions": {
            "uz": "Mol go'shtidan shashlik juda ham mazzali qiyma",
            "ru": "Очень вкусный шашлык из говядины",
            "en": "Very delicious beef barbecue"
        },
        "category": "shashlik",
        "price": 25000,
        "isThere": True,
        "imageUrl": "https://example.com/moloti.jpg",
        "ingredients": {
            "uz": ["Mol go'shti", "Piyoz", "Ziravorlar"],
            "ru": ["Говядина", "Лук", "Специи"],
            "en": ["Beef", "Onion", "Spices"]
        },
        "allergens": [],
        "rating": 4.5,
        "review_count": 15,
        "preparation_time": 20
    },
    "amur_6": {
        "id": "amur_6",
        "names": {
            "uz": "Tovuq Shashlik 2",
            "ru": "Куриный шашлык 2",
            "en": "Chicken Barbecue 2"
        },
        "descriptions": {
            "uz": "Yumshoq tovuq go'shtidan tayyorlangan shashlik",
            "ru": "Шашлык из нежного куриного мяса",
            "en": "Barbecue made from tender chicken meat"
        },
        "category": "shashlik",
        "price": 18000,
        "isThere": True,
        "imageUrl": "https://example.com/tovuq.jpg",
        "ingredients": {
            "uz": ["Tovuq go'shti", "Piyoz", "Ziravorlar"],
            "ru": ["Курица", "Лук", "Специи"],
            "en": ["Chicken", "Onion", "Spices"]
        },
        "allergens": [],
        "rating": 4.2,
        "review_count": 12,
        "preparation_time": 15
    },
    "amur_7": {
        "id": "amur_7",
        "names": {
            "uz": "Osh 2",
            "ru": "Плов 2",
            "en": "Pilaf 2"
        },
        "descriptions": {
            "uz": "An'anaviy o'zbek oshi, mol go'shti bilan",
            "ru": "Традиционный узбекский плов с говядиной",
            "en": "Traditional Uzbek pilaf with beef"
        },
        "category": "milliy_taomlar",
        "price": 15000,
        "isThere": True,
        "imageUrl": "https://example.com/osh.jpg",
        "ingredients": {
            "uz": ["Guruch", "Mol go'shti", "Sabzi", "Piyoz"],
            "ru": ["Рис", "Говядина", "Морковь", "Лук"],
            "en": ["Rice", "Beef", "Carrot", "Onion"]
        },
        "allergens": [],
        "rating": 4.8,
        "review_count": 25,
        "preparation_time": 30
    },
    "amur_8": {
        "id": "amur_8",
        "names": {
            "uz": "Lag'mon 2",
            "ru": "Лагман 2",
            "en": "Lagman 2"
        },
        "descriptions": {
            "uz": "An'anaviy o'zbek lag'moni, sabzavat bilan",
            "ru": "Традиционный узбекский лагман с овощами",
            "en": "Traditional Uzbek lagman with vegetables"
        },
        "category": "milliy_taomlar",
        "price": 12000,
        "isThere": True,
        "imageUrl": "https://example.com/lagmon.jpg",
        "ingredients": {
            "uz": ["Makaron", "Mol go'shti", "Sabzavotlar"],
            "ru": ["Лапша", "Говядина", "Овощи"],
            "en": ["Noodles", "Beef", "Vegetables"]
        },
        "allergens": {
            "uz": ["Gluten"],
            "ru": ["Глютен"],
            "en": ["Gluten"]
        },
        "rating": 4.3,
        "review_count": 18,
        "preparation_time": 25
    },
    "amur_9": {
        "id": "amur_9",
        "names": {
            "uz": "Somsa",
            "ru": "Самса",
            "en": "Samsa"
        },
        "descriptions": {
            "uz": "An'anaviy o'zbek somsasi, go'sht bilan",
            "ru": "Традиционная узбекская самса с мясом",
            "en": "Traditional Uzbek samsa with meat"
        },
        "category": "milliy_taomlar",
        "price": 8000,
        "isThere": True,
        "imageUrl": "https://example.com/somsa.jpg",
        "ingredients": {
            "uz": ["Xamir", "Mol go'shti", "Piyoz"],
            "ru": ["Тесто", "Говядина", "Лук"],
            "en": ["Dough", "Beef", "Onion"]
        },
        "allergens": {
            "uz": ["Gluten"],
            "ru": ["Глютен"],
            "en": ["Gluten"]
        },
        "rating": 4.6,
        "review_count": 22,
        "preparation_time": 25
    },
    "amur_10": {
        "id": "amur_10",
        "names": {
            "uz": "Manti",
            "ru": "Манты",
            "en": "Manti"
        },
        "descriptions": {
            "uz": "Bug'da pishirilgan manti, go'sht bilan",
            "ru": "Приготовленные на пару манты с мясом",
            "en": "Steamed manti with meat"
        },
        "category": "milliy_taomlar",
        "price": 20000,
        "isThere": True,
        "imageUrl": "https://example.com/manti.jpg",
        "ingredients": {
            "uz": ["Xamir", "Mol go'shti", "Piyoz", "Ziravorlar"],
            "ru": ["Тесто", "Говядина", "Лук", "Специи"],
            "en": ["Dough", "Beef", "Onion", "Spices"]
        },
        "allergens": {
            "uz": ["Gluten"],
            "ru": ["Глютен"],
            "en": ["Gluten"]
        },
        "rating": 4.7,
        "review_count": 30,
        "preparation_time": 35
    }
}

# Ma'lumotlar bazasi (test uchun xotirada)
USERS_DB = {
    "770451117": {
        "id": "user_1",
        "number": "770451117",
        "password": hashlib.md5("samandar".encode()).hexdigest(),
        "role": "admin",
        "full_name": "Samandar Admin",
        "email": "admin@restaurant.uz",
        "created_at": "2024-01-01 00:00:00",
        "is_active": True,
        "tg_id": 1713329317,
        "language": "uz"
    },
    "998901234567": {
        "id": "user_2",
        "number": "998901234567",
        "password": hashlib.md5("user123".encode()).hexdigest(),
        "role": "user",
        "full_name": "Test User",
        "email": "user@test.uz",
        "created_at": "2024-01-01 00:00:00",
        "is_active": True,
        "tg_id": 1066137436,
        "language": "uz"
    }
}

# Asosiy ovqatlar bazasi (eski format, ko'p tilli bilan birga ishlaydi)
FOODS_DB = {
    "amur_1": {
        "id": "amur_1",
        "name": "Moloti",
        "category": "Shashlik",
        "price": 23000,
        "description": "Mol Go'shtidan shashlik juda ham mazzali qiyma",
        "isThere": True,
        "imageUrl": "https://example.com/moloti.jpg",
        "ingredients": ["Mol go'shti", "Piyoz", "Ziravorlar"],
        "allergens": [],
        "rating": 4.5,
        "review_count": 15,
        "preparation_time": 20
    },
    "amur_2": {
        "id": "amur_2",
        "name": "Tovuq Shashlik",
        "category": "Shashlik",
        "price": 18000,
        "description": "Yumshoq tovuq go'shtidan tayyorlangan shashlik",
        "isThere": True,
        "imageUrl": "https://example.com/tovuq.jpg",
        "ingredients": ["Tovuq go'shti", "Piyoz", "Ziravorlar"],
        "allergens": [],
        "rating": 4.2,
        "review_count": 12,
        "preparation_time": 15
    },
    "amur_3": {
        "id": "amur_3",
        "name": "Osh",
        "category": "Milliy taomlar",
        "price": 15000,
        "description": "An'anaviy o'zbek oshi, mol go'shti bilan",
        "isThere": True,
        "imageUrl": "https://example.com/osh.jpg",
        "ingredients": ["Guruch", "Mol go'shti", "Sabzi", "Piyoz"],
        "allergens": [],
        "rating": 4.8,
        "review_count": 25,
        "preparation_time": 30
    },
    # Asosiy ovqatlar bazasini yangilash (eski format - test uchun)
    # Bu ovqatlar MULTILINGUAL_FOODS_DB da mavjud emas
    "amur_11": {
        "id": "amur_11",
        "name": "Norin",
        "category": "Milliy taomlar",
        "price": 14000,
        "description": "An'anaviy o'zbek norini, ot go'shti bilan",
        "isThere": True,
        "imageUrl": "https://example.com/norin.jpg",
        "ingredients": ["Xamir", "Ot go'shti", "Piyoz"],
        "allergens": ["Gluten"],
        "rating": 4.4,
        "review_count": 18,
        "preparation_time": 30
    },
    "amur_12": {
        "id": "amur_12",
        "name": "Sho'rva",
        "category": "Milliy taomlar",
        "price": 10000,
        "description": "Issiq sho'rva, sabzavotlar bilan",
        "isThere": True,
        "imageUrl": "https://example.com/shorva.jpg",
        "ingredients": ["Mol go'shti", "Sabzavotlar", "Ziravorlar"],
        "allergens": [],
        "rating": 4.1,
        "review_count": 14,
        "preparation_time": 25
    }
}

# Buyurtmalar bazasi
ORDERS_DB = {}

# Restoran stollar bazasi
RESTAURANT_TABLES = {
    "Zal-1 Stol-1": "93e05d01c3304b3b9dc963db187dbb51",
    "Zal-1 Stol-2": "73d6827a734a43b6ad779b5979bb9c6a",
    "Zal-1 Stol-3": "dc6e76e87f9e42a08a4e1198fc5f89a0",
    "Zal-1 Stol-4": "70a53b0ac3264fce88d9a4b7d3a7fa5e",
}

# Yangi ma'lumotlar bazalari
REVIEWS_DB = {}
NOTIFICATIONS_DB = {}
PROMOTIONS_DB = {}
INVENTORY_DB = {}
STAFF_DB = {}
TICKETS_DB = {}

# Kunlik buyurtmalar hisoblagichi
DAILY_ORDER_COUNTER = {}

# Email sozlamalari
EMAIL_CONFIG = {
    "smtp_server": "smtp.gmail.com",
    "smtp_port": 587,
    "email": "your_email@gmail.com",
    "password": "your_app_password"
}

# Sozlamalar
SETTINGS = {
    "restaurant": {
        "name": "Amur Restaurant",
        "address": "Toshkent, O'zbekiston",
        "phone": "+998901234567",
        "email": "info@amur-restaurant.uz",
        "working_hours": {
            "monday": "09:00-23:00",
            "tuesday": "09:00-23:00",
            "wednesday": "09:00-23:00",
            "thursday": "09:00-23:00",
            "friday": "09:00-24:00",
            "saturday": "09:00-24:00",
            "sunday": "10:00-23:00"
        },
        "delivery_fee": 5000,
        "min_order_amount": 25000,
        "max_delivery_distance": 15
    }
}

# Ko'p tilli yordamchi funksiyalar
def get_localized_food(food_id: str, lang: str = "uz") -> dict:
    """Ko'p tilli ovqat ma'lumotlarini olish"""
    # Avval ko'p tilli bazadan izlash
    if food_id in MULTILINGUAL_FOODS_DB:
        food = MULTILINGUAL_FOODS_DB[food_id].copy()
        
        # Til bo'yicha ma'lumotlarni olish
        food["name"] = food["names"].get(lang, food["names"]["uz"])
        food["description"] = food["descriptions"].get(lang, food["descriptions"]["uz"])
        food["ingredients"] = food["ingredients"].get(lang, food["ingredients"]["uz"])
        
        # Allergenlarni ham tarjima qilish
        if isinstance(food.get("allergens"), dict):
            food["allergens"] = food["allergens"].get(lang, food["allergens"].get("uz", []))
        elif not food.get("allergens"):
            food["allergens"] = []
        
        # Kategoriya nomini tarjima qilish
        food["category_name"] = get_translation(food["category"], lang)
        
        # Kerak bo'lmagan kalitlarni olib tashlash
        food.pop("names", None)
        food.pop("descriptions", None)
        
        return food
    
    # Agar ko'p tilli bazada yo'q bo'lsa, asosiy bazadan olish va tarjima qilish
    elif food_id in FOODS_DB:
        food = FOODS_DB[food_id].copy()
        
        # Asosiy bazadagi ovqatni ko'p tilli qilish (default o'zbek tili)
        if lang != "uz":
            # Bu yerda siz qo'lda tarjima qilishingiz mumkin yoki default qiymatlani qoldirish mumkin
            # Hozircha asl nomlarni qoldiramiz
            pass
        
        food["category_name"] = get_translation(food["category"].lower().replace(" ", "_"), lang)
        return food
    
    return None

def get_all_localized_foods(lang: str = "uz") -> list:
    """Barcha ovqatlarni ko'p tilli olish"""
    localized_foods = []
    
    # Birinchi navbatda ko'p tilli bazadan barcha ovqatlarni olish
    for food_id in MULTILINGUAL_FOODS_DB.keys():
        food = get_localized_food(food_id, lang)
        if food:
            localized_foods.append(food)
    
    # Keyin qolgan ovqatlarni asosiy bazadan olish (ko'p tilli bazada yo'q bo'lganlarni)
    for food_id in FOODS_DB.keys():
        if food_id not in MULTILINGUAL_FOODS_DB:
            food = get_localized_food(food_id, lang)
            if food:
                localized_foods.append(food)
    
    return localized_foods

# Telegram yordamchi funksiyalar
async def send_telegram_message(chat_id: int, message: str):
    """Telegram orqali xabar yuborish"""
    try:
        async with aiohttp.ClientSession() as session:
            data = {
                "chat_id": chat_id,
                "text": message,
                "parse_mode": "HTML"
            }
            async with session.post(TELEGRAM_API_URL, json=data) as response:
                if response.status == 200:
                    logger.info(f"Telegram xabar yuborildi: {chat_id}")
                    return True
                else:
                    logger.error(f"Telegram xabar yuborishda xato: {response.status}")
                    return False
    except Exception as e:
        logger.error(f"Telegram API xatosi: {e}")
        return False

def format_order_for_telegram(order: dict, lang: str = "uz") -> str:
    """Buyurtmani Telegram uchun ko'p tilli formatlash"""
    foods_list = ""
    for food in order["foods"]:
        foods_list += f"• {food['name']} x{food['count']} = {food['total_price']:,} so'm\n"
    
    delivery_info = ""
    if order["delivery_type"] == "delivery":
        delivery_info = f"📍 {get_translation('delivery_address', lang)}: {order['delivery_info']['location']}"
    elif order["delivery_type"] == "own_withdrawal":
        delivery_info = f"🏃‍♂️ {get_translation('pickup', lang)}: {order['delivery_info']['pickup_code']}"
    elif order["delivery_type"] == "atTheRestaurant":
        delivery_info = f"🍽 {get_translation('restaurant_table', lang)}: {order['delivery_info']['table_name']}"
    
    message = f"""
🍽 <b>{get_translation('new_order', lang)}</b>

📋 <b>{get_translation('order_id', lang)}</b> {order['order_id']}
👤 <b>{get_translation('customer', lang)}</b> {order['user_name']}
📞 <b>{get_translation('phone', lang)}</b> {order['user_number']}
📅 <b>{get_translation('time', lang)}</b> {order['order_time']}

<b>{get_translation('order_items', lang)}</b>
{foods_list}
💰 <b>{get_translation('total_amount', lang)}</b> {order['total_price']:,} so'm

{delivery_info}
💳 <b>{get_translation('payment_method', lang)}</b> {get_translation(order['payment_info']['method'], lang)}
⏱ <b>{get_translation('preparation_time', lang)}</b> {order.get('estimated_time', 'Nomalum')} daqiqa

{f"📝 <b>{get_translation('additional_notes', lang)}</b> {order['special_instructions']}" if order.get('special_instructions') else ""}
"""
    return message

def format_notification_for_telegram(title: str, message: str, order_id: str = None, lang: str = "uz") -> str:
    """Bildirishnomani Telegram uchun formatlash"""
    if order_id:
        return f"🔔 <b>{title}</b>\n\n{message}\n\n📋 {get_translation('order_id', lang)} {order_id}"
    else:
        return f"🔔 <b>{title}</b>\n\n{message}"

# Yordamchi funksiyalar
def create_access_token(data: dict):
    to_encode = data.copy()
    expire = datetime.utcnow() + timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES)
    to_encode.update({"exp": expire})
    encoded_jwt = jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)
    return encoded_jwt

def verify_token(credentials: HTTPAuthorizationCredentials = Depends(security)):
    try:
        payload = jwt.decode(credentials.credentials, SECRET_KEY, algorithms=[ALGORITHM])
        number: str = payload.get("sub")
        role: str = payload.get("role") 
        user_id: str = payload.get("user_id")
        if number is None:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Token yaroqsiz"
            )
        return {"number": number, "role": role, "user_id": user_id}
    except jwt.PyJWTError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token yaroqsiz"
        )

def generate_id(prefix: str = "id"):
    return f"{prefix}_{str(uuid.uuid4()).replace('-', '')[:8]}"

def generate_order_id():
    today = datetime.now().strftime("%Y-%m-%d")
    if today not in DAILY_ORDER_COUNTER:
        DAILY_ORDER_COUNTER[today] = 0
    DAILY_ORDER_COUNTER[today] += 1
    return f"{today}-{DAILY_ORDER_COUNTER[today]}"

def get_table_name_by_id(table_id: str) -> str:
    for table_name, t_id in RESTAURANT_TABLES.items():
        if t_id == table_id:
            return table_name
    return "Noma'lum stol"

def save_uploaded_file(file: UploadFile) -> str:
    allowed_extensions = [".jpg", ".jpeg", ".png", ".gif", ".webp"]
    file_extension = os.path.splitext(file.filename)[1].lower()
    
    if file_extension not in allowed_extensions:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Fayl formati qo'llab-quvvatlanmaydi. Ruxsat etilgan formatlar: {', '.join(allowed_extensions)}"
        )
    
    file_id = str(uuid.uuid4())
    new_filename = f"{file_id}{file_extension}"
    file_path = os.path.join(UPLOAD_DIR, new_filename)
    
    try:
        with open(file_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Fayl saqlanmadi: {str(e)}"
        )
    
    return f"/uploads/{new_filename}"

def calculate_food_rating(food_id: str):
    """Ovqat reytingini qayta hisoblash"""
    food_reviews = [r for r in REVIEWS_DB.values() if r["food_id"] == food_id]
    if not food_reviews:
        return 0.0, 0
    
    total_rating = sum(r["rating"] for r in food_reviews)
    avg_rating = round(total_rating / len(food_reviews), 1)
    return avg_rating, len(food_reviews)

def send_email_notification(to_email: str, subject: str, body: str):
    """Email yuborish"""
    try:
        msg = MIMEMultipart()
        msg['From'] = EMAIL_CONFIG["email"]
        msg['To'] = to_email
        msg['Subject'] = subject
        
        msg.attach(MIMEText(body, 'plain', 'utf-8'))
        
        server = smtplib.SMTP(EMAIL_CONFIG["smtp_server"], EMAIL_CONFIG["smtp_port"])
        server.starttls()
        server.login(EMAIL_CONFIG["email"], EMAIL_CONFIG["password"])
        text = msg.as_string()
        server.sendmail(EMAIL_CONFIG["email"], to_email, text)
        server.quit()
        return True
    except Exception as e:
        print(f"Email yuborishda xato: {e}")
        return False

def create_notification(user_id: str, title: str, message: str, notification_type: str = "system"):
    """Bildirishnoma yaratish"""
    notification_id = generate_id("notif")
    notification = Notification(
        id=notification_id,
        user_id=user_id,
        title=title,
        message=message,
        type=notification_type,
        created_at=datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    )
    NOTIFICATIONS_DB[notification_id] = notification.dict()
    return notification

async def send_order_notification(order_id: str, user_id: str, status: str):
    """Buyurtma holati haqida bildirishnoma yuborish"""
    # Foydalanuvchi tilini olish
    user_lang = get_user_language_preference(user_id)
    
    status_messages = {
        "confirmed": get_translation("order_confirmed", user_lang),
        "preparing": get_translation("order_preparing", user_lang),
        "ready": get_translation("order_ready", user_lang),
        "delivered": get_translation("order_delivered", user_lang),
        "cancelled": get_translation("order_cancelled_msg", user_lang)
    }
    
    if status in status_messages:
        # Lokal bildirishnoma yaratish
        create_notification(
            user_id=user_id,
            title=f"{get_translation('order_id', user_lang)} #{order_id}",
            message=status_messages[status],
            notification_type="order"
        )
        
        # Telegram orqali yuborish
        user = None
        for u in USERS_DB.values():
            if u["id"] == user_id:
                user = u
                break
        
        if user and user.get("tg_id"):
            telegram_message = format_notification_for_telegram(
                f"{get_translation('order_id', user_lang)} #{order_id}",
                status_messages[status],
                order_id,
                user_lang
            )
            await send_telegram_message(user["tg_id"], telegram_message)

# Middleware: Til boshqaruvi
@app.middleware("http")
async def language_middleware(request: Request, call_next):
    """Har bir request uchun tilni tekshirish"""
    response = await call_next(request)
    return response

# API endpointlari

# ========== TIL SOZLAMALARI ==========
@app.post("/api/settings/language", tags=["Sozlamalar"])
def set_language(language_request: LanguageRequest, current_user: dict = Depends(verify_token)):
    """Foydalanuvchi tilini o'rnatish"""
    lang = language_request.language
    if lang not in ["uz", "ru", "en"]:
        raise HTTPException(
            status_code=400, 
            detail="Qo'llab-quvvatlanadigan tillar: uz, ru, en"
        )
    
    # Foydalanuvchi bazasida tilni yangilash
    user = USERS_DB.get(current_user["number"])
    if user:
        user["language"] = lang
        USERS_DB[current_user["number"]] = user
    
    # Xotirada saqlash
    success = set_user_language(current_user["user_id"], lang)
    if success:
        return create_response("success", lang)
    else:
        raise HTTPException(status_code=400, detail="Til o'rnatishda xatolik")

@app.get("/api/settings/language", tags=["Sozlamalar"])
def get_language_info(current_user: dict = Depends(verify_token)):
    """Foydalanuvchi til sozlamalarini olish"""
    user_lang = get_user_language_preference(current_user["user_id"])
    
    return {
        "current_language": user_lang,
        "available_languages": [
            {"code": "uz", "name": "O'zbek"},
            {"code": "ru", "name": "Русский"},
            {"code": "en", "name": "English"}
        ]
    }

@app.get("/api/categories", tags=["Kategoriyalar"])
def get_categories(request: Request):
    """Ko'p tilli kategoriyalar ro'yxati"""
    lang = get_user_language(dict(request.headers))
    
    categories = [
        {
            "key": "shashlik",
            "name": get_translation("shashlik", lang)
        },
        {
            "key": "milliy_taomlar", 
            "name": get_translation("milliy_taomlar", lang)
        },
        {
            "key": "ichimliklar",
            "name": get_translation("ichimliklar", lang)
        },
        {
            "key": "salatlar",
            "name": get_translation("salatlar", lang)
        },
        {
            "key": "shirinliklar",
            "name": get_translation("shirinliklar", lang)
        }
    ]
    
    return categories

@app.get("/api/search", tags=["Qidiruv"])
def search_multilingual(
    q: str,
    request: Request,
    category: Optional[str] = None
):
    """Ko'p tilli qidiruv"""
    lang = get_user_language(dict(request.headers))
    
    foods = get_all_localized_foods(lang)
    
    # Qidiruv
    search_lower = q.lower()
    results = []
    
    for food in foods:
        # Nom, tavsif va ingredientlarda qidirish
        if (search_lower in food["name"].lower() or 
            search_lower in food["description"].lower() or
            any(search_lower in ingredient.lower() for ingredient in food.get("ingredients", []))):
            results.append(food)
    
    # Kategoriya filtri
    if category:
        results = [f for f in results if f["category"] == category]
    
    return {
        "query": q,
        "language": lang,
        "results": results,
        "total": len(results)
    }

# ========== AUTENTIFIKATSIYA ==========
@app.post("/api/register", response_model=LoginResponse, tags=["Autentifikatsiya"])
def register(request: RegisterRequest):
    """Yangi foydalanuvchi ro'yxatdan o'tkazish"""
    lang = request.language or "uz"
    
    if request.number in USERS_DB:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=get_translation("phone_already_registered", lang)
        )
    
    user_id = generate_id("user")
    password_hash = hashlib.md5(request.password.encode()).hexdigest()
    
    new_user = {
        "id": user_id,
        "number": request.number,
        "password": password_hash,
        "role": "user",
        "full_name": request.full_name,
        "email": request.email,
        "created_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "is_active": True,
        "tg_id": request.tg_id,
        "language": lang
    }
    
    USERS_DB[request.number] = new_user
    
    # Foydalanuvchi tilini saqlash
    set_user_language(user_id, lang)
    
    token = create_access_token({
        "sub": request.number, 
        "role": "user",
        "user_id": user_id
    })
    
    return LoginResponse(token=token, role="user", user_id=user_id, language=lang)

@app.post("/api/login", response_model=LoginResponse, tags=["Autentifikatsiya"])
def login(request: LoginRequest):
    """Foydalanuvchi tizimga kirishi"""
    user = USERS_DB.get(request.number)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=get_translation("invalid_credentials", "uz")
        )
    
    password_hash = hashlib.md5(request.password.encode()).hexdigest()
    if user["password"] != password_hash:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=get_translation("invalid_credentials", user.get("language", "uz"))
        )
    
    user_lang = user.get("language", "uz")
    set_user_language(user["id"], user_lang)
    
    token = create_access_token({
        "sub": request.number, 
        "role": user["role"],
        "user_id": user["id"]
    })
    
    return LoginResponse(token=token, role=user["role"], user_id=user["id"], language=user_lang)

@app.get("/api/profile", response_model=User, tags=["Foydalanuvchi"])
def get_profile(current_user: dict = Depends(verify_token)):
    """Foydalanuvchi profilini olish"""
    user = USERS_DB.get(current_user["number"])
    if not user:
        raise HTTPException(status_code=404, detail=get_translation("user_not_found", "uz"))
    
    user_copy = user.copy()
    user_copy.pop("password", None)
    return user_copy

# ========== OVQATLAR (Ko'p tilli) ==========
@app.get("/api/foods", response_model=List[Food], tags=["Ovqatlar"])
def get_all_foods(
    request: Request,
    category: Optional[str] = None, 
    search: Optional[str] = None
):
    """Barcha ovqatlar ro'yxatini olish (ko'p tilli)"""
    lang = get_user_language(dict(request.headers))
    foods = get_all_localized_foods(lang)
    
    if category:
        foods = [f for f in foods if f["category"].lower() == category.lower()]
    
    if search:
        search_lower = search.lower()
        foods = [f for f in foods if search_lower in f["name"].lower() or search_lower in f["description"].lower()]
    
    return foods

@app.get("/api/foods/localized", tags=["Ovqatlar"])
def get_localized_foods(
    request: Request,
    category: Optional[str] = None, 
    search: Optional[str] = None
):
    """Ko'p tilli ovqatlar ro'yxati"""
    lang = get_user_language(dict(request.headers))
    foods = get_all_localized_foods(lang)
    
    if category:
        foods = [f for f in foods if f["category"] == category]
    
    if search:
        search_lower = search.lower()
        foods = [f for f in foods if search_lower in f["name"].lower() or search_lower in f["description"].lower()]
    
    return foods

@app.get("/api/foods/popular", response_model=List[Food], tags=["Ovqatlar"])
def get_popular_foods(request: Request, limit: int = 5):
    """Mashhur ovqatlarni olish (ko'p tilli)"""
    lang = get_user_language(dict(request.headers))
    foods = get_all_localized_foods(lang)
    foods.sort(key=lambda x: (x.get("rating", 0), x.get("review_count", 0)), reverse=True)
    return foods[:limit]

@app.get("/api/foods/{food_id}", response_model=Food, tags=["Ovqatlar"])
def get_food(food_id: str, request: Request):
    """Bitta ovqat ma'lumotlari (ko'p tilli)"""
    lang = get_user_language(dict(request.headers))
    food = get_localized_food(food_id, lang)
    if not food:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("food_not_found", lang)
        )
    return food

@app.post("/api/foods", response_model=Food, tags=["Ovqatlar"])
def create_food(food: FoodCreate, request: Request, current_user: dict = Depends(verify_token)):
    """Yangi ovqat qo'shish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    food_id = generate_id("food")
    
    # Yangi ovqatni FOODS_DB ga qo'shish (eski format)
    new_food_dict = {
        "id": food_id,
        "name": food.name,
        "category": food.category,
        "price": food.price,
        "description": food.description,
        "isThere": food.isThere,
        "imageUrl": food.imageUrl,
        "ingredients": food.ingredients or [],
        "allergens": food.allergens or [],
        "rating": 0.0,
        "review_count": 0,
        "preparation_time": food.preparation_time
    }
    
    FOODS_DB[food_id] = new_food_dict
    
    # Response uchun Food obyektini yaratish
    new_food = Food(
        id=food_id,
        name=food.name,
        category=food.category,
        price=food.price,
        description=food.description,
        isThere=food.isThere,
        imageUrl=food.imageUrl,
        ingredients=food.ingredients,
        allergens=food.allergens,
        preparation_time=food.preparation_time,
        category_name=get_translation(food.category.lower().replace(" ", "_"), lang)
    )
    
    return new_food

# Ko'p tilli ovqat qo'shish uchun yangi endpoint
@app.post("/api/foods/multilingual", tags=["Ovqatlar"])
def create_multilingual_food(request: Request, current_user: dict = Depends(verify_token)):
    """Ko'p tilli ovqat qo'shish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    # Bu endpoint uchun alohida model yaratish kerak
    # Hozircha basic response
    return {
        "message": get_translation("food_created", lang),
        "info": "Ko'p tilli ovqat qo'shish uchun MULTILINGUAL_FOODS_DB ga qo'lda qo'shish kerak"
    }

# Yangi model ko'p tilli ovqat uchun
class MultilingualFoodCreate(BaseModel):
    names: Dict[str, str]  # {"uz": "Osh", "ru": "Плов", "en": "Pilaf"}
    descriptions: Dict[str, str]
    ingredients: Dict[str, List[str]]
    category: str
    price: int
    isThere: bool = True
    imageUrl: str
    allergens: Optional[Dict[str, List[str]]] = None
    preparation_time: Optional[int] = 15

@app.post("/api/foods/multilingual/full", tags=["Ovqatlar"])
def create_full_multilingual_food(
    food: MultilingualFoodCreate, 
    request: Request, 
    current_user: dict = Depends(verify_token)
):
    """To'liq ko'p tilli ovqat qo'shish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    food_id = generate_id("food")
    
    # Ko'p tilli ovqatni MULTILINGUAL_FOODS_DB ga qo'shish
    multilingual_food = {
        "id": food_id,
        "names": food.names,
        "descriptions": food.descriptions,
        "ingredients": food.ingredients,
        "category": food.category,
        "price": food.price,
        "isThere": food.isThere,
        "imageUrl": food.imageUrl,
        "allergens": food.allergens or {},
        "rating": 0.0,
        "review_count": 0,
        "preparation_time": food.preparation_time
    }
    
    MULTILINGUAL_FOODS_DB[food_id] = multilingual_food
    
    # Response uchun foydalanuvchi tilida qaytarish
    localized_food = get_localized_food(food_id, lang)
    
    return {
        "message": get_translation("food_created", lang),
        "food": localized_food
    }

@app.put("/api/foods/{food_id}", response_model=Food, tags=["Ovqatlar"])
def update_food(food_id: str, food_update: FoodUpdate, request: Request, current_user: dict = Depends(verify_token)):
    """Ovqat ma'lumotlarini yangilash (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    food = FOODS_DB.get(food_id)
    if not food:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("food_not_found", lang)
        )
    
    # Ma'lumotlarni yangilash
    update_data = food_update.dict(exclude_unset=True)
    for field, value in update_data.items():
        food[field] = value
    
    FOODS_DB[food_id] = food
    return food

@app.delete("/api/foods/{food_id}", tags=["Ovqatlar"])
def delete_food(food_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Ovqatni o'chirish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    if food_id not in FOODS_DB:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("food_not_found", lang)
        )
    
    del FOODS_DB[food_id]
    return {"message": get_translation("food_deleted", lang)}

@app.post("/api/foods/upload-image", tags=["Ovqatlar"])
def upload_food_image(file: UploadFile = File(...), request: Request = None, current_user: dict = Depends(verify_token)):
    """Ovqat rasmi yuklash"""
    lang = get_user_language(dict(request.headers)) if request else "uz"
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("only_admin_can_manage_food", lang)
        )
    
    try:
        image_url = save_uploaded_file(file)
        return {
            "imageUrl": image_url, 
            "message": get_translation("image_uploaded", lang)
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ========== SHARHLAR ==========
@app.post("/api/reviews", response_model=Review, tags=["Sharhlar"])
def create_review(review: ReviewCreate, request: Request, current_user: dict = Depends(verify_token)):
    """Ovqat uchun sharh qoldirish"""
    lang = get_user_language(dict(request.headers))
    
    if review.food_id not in FOODS_DB and review.food_id not in MULTILINGUAL_FOODS_DB:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("food_not_found", lang)
        )
    
    # Bir foydalanuvchi bir ovqat uchun faqat bitta sharh qoldira oladi
    existing_review = None
    for r in REVIEWS_DB.values():
        if r["user_id"] == current_user["user_id"] and r["food_id"] == review.food_id:
            existing_review = r
            break
    
    if existing_review:
        raise HTTPException(
            status_code=400, 
            detail=get_translation("already_reviewed", lang)
        )
    
    review_id = generate_id("review")
    new_review = Review(
        id=review_id,
        user_id=current_user["user_id"],
        food_id=review.food_id,
        rating=review.rating,
        comment=review.comment,
        created_at=datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    )
    
    REVIEWS_DB[review_id] = new_review.dict()
    
    # Ovqat reytingini yangilash
    rating, count = calculate_food_rating(review.food_id)
    if review.food_id in FOODS_DB:
        FOODS_DB[review.food_id]["rating"] = rating
        FOODS_DB[review.food_id]["review_count"] = count
    if review.food_id in MULTILINGUAL_FOODS_DB:
        MULTILINGUAL_FOODS_DB[review.food_id]["rating"] = rating
        MULTILINGUAL_FOODS_DB[review.food_id]["review_count"] = count
    
    return new_review

@app.get("/api/foods/{food_id}/reviews", response_model=List[Review], tags=["Sharhlar"])
def get_food_reviews(food_id: str):
    """Ovqat sharhlari"""
    reviews = [r for r in REVIEWS_DB.values() if r["food_id"] == food_id]
    reviews.sort(key=lambda x: x["created_at"], reverse=True)
    return reviews

@app.get("/api/reviews/{review_id}", response_model=Review, tags=["Sharhlar"])
def get_review(review_id: str, request: Request):
    """Bitta sharh ma'lumotlari"""
    lang = get_user_language(dict(request.headers))
    
    review = REVIEWS_DB.get(review_id)
    if not review:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("review_not_found", lang)
        )
    return review

@app.delete("/api/reviews/{review_id}", tags=["Sharhlar"])
def delete_review(review_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Sharhni o'chirish"""
    lang = get_user_language(dict(request.headers))
    
    review = REVIEWS_DB.get(review_id)
    if not review:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("review_not_found", lang)
        )
    
    # Faqat sharh egasi yoki admin o'chira oladi
    if review["user_id"] != current_user["user_id"] and current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    food_id = review["food_id"]
    del REVIEWS_DB[review_id]
    
    # Ovqat reytingini qayta hisoblash
    rating, count = calculate_food_rating(food_id)
    if food_id in FOODS_DB:
        FOODS_DB[food_id]["rating"] = rating
        FOODS_DB[food_id]["review_count"] = count
    if food_id in MULTILINGUAL_FOODS_DB:
        MULTILINGUAL_FOODS_DB[food_id]["rating"] = rating
        MULTILINGUAL_FOODS_DB[food_id]["review_count"] = count
    
    return {"message": get_translation("review_deleted", lang)}

# ========== BUYURTMALAR (kengaytirilgan) ==========
@app.post("/api/orders", response_model=Order, tags=["Buyurtmalar"])
async def create_order(
    order_request: OrderRequest, 
    request: Request,
    background_tasks: BackgroundTasks,
    current_user: dict = Depends(verify_token)
):
    """Yangi buyurtma berish (kengaytirilgan)"""
    lang = get_user_language(dict(request.headers))
    
    # Ovqatlarni tekshirish
    ordered_foods = []
    total_price = 0
    total_prep_time = 0
    
    food_orders = {}
    for food_dict in order_request.food_ids:
        for food_id, count in food_dict.items():
            food_orders[food_id] = count
    
    for food_id, count in food_orders.items():
        if count <= 0:
            raise HTTPException(
                status_code=400, 
                detail=f"{get_translation('invalid_quantity', lang)}: {food_id}"
            )
        
        # Ko'p tilli ovqat ma'lumotlarini olish
        food = get_localized_food(food_id, lang)
        if not food or not food["isThere"]:
            raise HTTPException(
                status_code=400, 
                detail=f"{get_translation('food_not_available', lang)}: {food_id}"
            )
        
        food_total_price = food["price"] * count
        prep_time = food.get("preparation_time", 15)
        total_prep_time = max(total_prep_time, prep_time)
        
        # Ko'p tilli OrderFood yaratish
        ordered_food = OrderFood(
            id=food["id"],
            name=food["name"],  # Ko'p tilli nom
            category=food.get("category_name", food["category"]),  # Ko'p tilli kategoriya
            price=food["price"],
            description=food["description"],  # Ko'p tilli tavsif
            imageUrl=food["imageUrl"],
            count=count,
            total_price=food_total_price
        )
        ordered_foods.append(ordered_food)
        total_price += food_total_price
    
    # Yetkazib berish ma'lumotlari
    delivery_type = ""
    delivery_info = {}
    
    if "delivery" in order_request.to_give:
        delivery_type = "delivery"
        delivery_info = {
            "type": "delivery",
            "location": order_request.to_give["delivery"]
        }
        total_prep_time += 20  # yetkazib berish vaqti
    elif "own_withdrawal" in order_request.to_give:
        delivery_type = "own_withdrawal"
        delivery_info = {
            "type": "own_withdrawal",
            "pickup_code": order_request.to_give["own_withdrawal"]
        }
    elif "atTheRestaurant" in order_request.to_give:
        delivery_type = "atTheRestaurant"
        table_id = order_request.to_give["atTheRestaurant"]
        table_name = get_table_name_by_id(table_id)
        delivery_info = {
            "type": "atTheRestaurant",
            "table_id": table_id,
            "table_name": table_name
        }
    
    # To'lov ma'lumotlari
    payment_info = PaymentInfo(
        method=order_request.payment_method,
        amount=total_price,
        transaction_id=generate_id("txn") if order_request.payment_method != PaymentMethod.CASH else None
    )
    
    # Buyurtma yaratish
    order_id = generate_order_id()
    order_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    user = USERS_DB.get(current_user["number"])
    user_name = user.get("full_name", "Foydalanuvchi") if user else "Foydalanuvchi"
    
    new_order = Order(
        order_id=order_id,
        user_number=current_user["number"],
        user_name=user_name,
        foods=ordered_foods,  # Ko'p tilli ovqatlar
        total_price=total_price,
        order_time=order_time,
        delivery_type=delivery_type,
        delivery_info=delivery_info,
        payment_info=payment_info,
        special_instructions=order_request.special_instructions,
        estimated_time=total_prep_time
    )
    
    ORDERS_DB[order_id] = new_order.dict()
    
    # Bildirishnoma yuborish
    background_tasks.add_task(
        send_order_notification, 
        order_id, 
        current_user["user_id"], 
        "confirmed"
    )
    
    # Adminlarga va foydalanuvchiga Telegram orqali xabar yuborish
    order_dict = new_order.dict()
    
    # Adminlarga yuborish (har birining tilida)
    for admin_user in USERS_DB.values():
        if admin_user["role"] == "admin" and admin_user.get("tg_id"):
            admin_lang = admin_user.get("language", "uz")
            
            # Admin uchun ovqatlarni uning tilida formatlash
            admin_order_dict = await format_order_for_admin(order_dict, admin_lang)
            telegram_message = format_order_for_telegram(admin_order_dict, admin_lang)
            background_tasks.add_task(
                send_telegram_message,
                admin_user["tg_id"],
                telegram_message
            )
    
    # Foydalanuvchiga tasdiqlash xabari (foydalanuvchi tilida)
    if user and user.get("tg_id"):
        user_lang = user.get("language", "uz")
        
        # Foydalanuvchi uchun ovqatlarni uning tilida formatlash
        user_order_dict = await format_order_for_user(order_dict, user_lang)
        
        # Buyurtma tarkibini formatlash
        foods_detail = ""
        for food in user_order_dict["foods"]:
            foods_detail += f"• {food['name']} x{food['count']} = {food['total_price']:,} so'm\n"
        
        delivery_info_text = ""
        if delivery_type == "delivery":
            delivery_info_text = f"📍 {get_translation('delivery_address', user_lang)}: {delivery_info['location']}"
        elif delivery_type == "own_withdrawal":
            delivery_info_text = f"🏃‍♂️ {get_translation('pickup', user_lang)}: {delivery_info['pickup_code']}"
        elif delivery_type == "atTheRestaurant":
            delivery_info_text = f"🍽 {get_translation('restaurant_table', user_lang)}: {delivery_info['table_name']}"
        
        user_message = f"""
✅ <b>{get_translation('order_accepted', user_lang)}</b>

📋 <b>{get_translation('order_id', user_lang)}</b> {order_id}
📅 <b>{get_translation('time', user_lang)}</b> {order_time}

<b>{get_translation('order_items', user_lang)}</b>
{foods_detail}
💰 <b>{get_translation('total_amount', user_lang)}</b> {total_price:,} so'm
⏱ <b>{get_translation('preparation_time', user_lang)}</b> {total_prep_time} daqiqa

{delivery_info_text}

{get_translation('order_status_notification', user_lang)}
        """
        background_tasks.add_task(
            send_telegram_message,
            user["tg_id"],
            user_message
        )
        
        logger.info(f"Foydalanuvchiga xabar yuborildi: {user['tg_id']}")
    else:
        logger.warning(f"Foydalanuvchi telegram ID topilmadi: {current_user['number']}")
    
    # Telegram guruhga ham yuborish (o'zbek tilida)
    group_order_dict = await format_order_for_admin(order_dict, "uz")
    telegram_message = format_order_for_telegram(group_order_dict, "uz")
    background_tasks.add_task(
        send_telegram_message,
        TELEGRAM_GROUP_ID,
        telegram_message
    )
    
    return new_order

# Ko'p tilli buyurtma formatlash funksiyalari
async def format_order_for_admin(order_dict: dict, lang: str) -> dict:
    """Admin uchun buyurtmani uning tilida formatlash"""
    formatted_order = order_dict.copy()
    formatted_foods = []
    
    for food in order_dict["foods"]:
        # Har bir ovqatni admin tili bo'yicha formatlash
        localized_food = get_localized_food(food["id"], lang)
        if localized_food:
            formatted_food = food.copy()
            formatted_food["name"] = localized_food["name"]
            formatted_food["description"] = localized_food["description"]
            formatted_food["category"] = localized_food.get("category_name", food["category"])
            formatted_foods.append(formatted_food)
        else:
            formatted_foods.append(food)
    
    formatted_order["foods"] = formatted_foods
    return formatted_order

async def format_order_for_user(order_dict: dict, lang: str) -> dict:
    """Foydalanuvchi uchun buyurtmani uning tilida formatlash"""
    return await format_order_for_admin(order_dict, lang)

@app.get("/api/orders", response_model=List[Order], tags=["Buyurtmalar"])
def get_orders(request: Request, current_user: dict = Depends(verify_token)):
    """Buyurtmalar ro'yxati (ko'p tilli)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] == "admin":
        # Admin barcha buyurtmalarni ko'radi
        orders = list(ORDERS_DB.values())
    else:
        # Oddiy foydalanuvchi faqat o'z buyurtmalarini ko'radi
        orders = [order for order in ORDERS_DB.values() if order["user_number"] == current_user["number"]]
    
    # Buyurtmalardagi ovqatlarni ko'p tilli qilish
    localized_orders = []
    for order in orders:
        localized_order = order.copy()
        localized_foods = []
        
        for food in order["foods"]:
            localized_food_data = get_localized_food(food["id"], lang)
            if localized_food_data:
                localized_food = food.copy()
                localized_food["name"] = localized_food_data["name"]
                localized_food["description"] = localized_food_data["description"]
                localized_food["category"] = localized_food_data.get("category_name", food["category"])
                localized_foods.append(localized_food)
            else:
                localized_foods.append(food)
        
        localized_order["foods"] = localized_foods
        localized_orders.append(localized_order)
    
    # Vaqt bo'yicha tartiblash (eng yangi birinchi)
    localized_orders.sort(key=lambda x: x["order_time"], reverse=True)
    return localized_orders

@app.get("/api/orders/{order_id}", response_model=Order, tags=["Buyurtmalar"])
def get_order(order_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Bitta buyurtma ma'lumotlari (ko'p tilli)"""
    lang = get_user_language(dict(request.headers))
    
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("order_not_found", lang)
        )
    
    # Foydalanuvchi faqat o'z buyurtmasini ko'ra oladi
    if current_user["role"] != "admin" and order["user_number"] != current_user["number"]:
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    # Buyurtmadagi ovqatlarni ko'p tilli qilish
    localized_order = order.copy()
    localized_foods = []
    
    for food in order["foods"]:
        localized_food_data = get_localized_food(food["id"], lang)
        if localized_food_data:
            localized_food = food.copy()
            localized_food["name"] = localized_food_data["name"]
            localized_food["description"] = localized_food_data["description"]
            localized_food["category"] = localized_food_data.get("category_name", food["category"])
            localized_foods.append(localized_food)
        else:
            localized_foods.append(food)
    
    localized_order["foods"] = localized_foods
    return localized_order

@app.put("/api/orders/{order_id}/status", tags=["Buyurtmalar"])
async def update_order_status(
    order_id: str, 
    new_status: OrderStatus,
    request: Request,
    background_tasks: BackgroundTasks,
    current_user: dict = Depends(verify_token)
):
    """Buyurtma holatini yangilash"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("order_not_found", lang)
        )
    
    order["status"] = new_status.value
    
    if new_status == OrderStatus.DELIVERED:
        order["delivered_at"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        # To'lovni tasdiqlash
        if order["payment_info"]["method"] == PaymentMethod.CASH:
            order["payment_info"]["status"] = PaymentStatus.PAID.value
            order["payment_info"]["payment_time"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    ORDERS_DB[order_id] = order
    
    # Foydalanuvchiga bildirishnoma
    user_id = None
    for user in USERS_DB.values():
        if user["number"] == order["user_number"]:
            user_id = user["id"]
            break
    
    if user_id:
        background_tasks.add_task(
            send_order_notification,
            order_id,
            user_id,
            new_status.value
        )
    
    return {"message": get_translation("order_status_updated", lang)}

@app.delete("/api/orders/{order_id}", tags=["Buyurtmalar"])
def cancel_order(order_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Buyurtmani bekor qilish"""
    lang = get_user_language(dict(request.headers))
    
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("order_not_found", lang)
        )
    
    # Faqat buyurtma egasi yoki admin bekor qila oladi
    if order["user_number"] != current_user["number"] and current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    # Faqat pending yoki confirmed holatdagi buyurtmalarni bekor qilish mumkin
    if order["status"] not in ["pending", "confirmed"]:
        raise HTTPException(
            status_code=400, 
            detail=get_translation("cannot_cancel_order", lang)
        )
    
    order["status"] = OrderStatus.CANCELLED.value
    ORDERS_DB[order_id] = order
    
    return {"message": get_translation("order_cancelled", lang)}

# ========== BILDIRISHNOMALAR ==========
@app.get("/api/notifications", response_model=List[Notification], tags=["Bildirishnomalar"])
def get_notifications(current_user: dict = Depends(verify_token)):
    """Foydalanuvchi bildirishnomalarini olish"""
    notifications = [n for n in NOTIFICATIONS_DB.values() if n["user_id"] == current_user["user_id"]]
    notifications.sort(key=lambda x: x["created_at"], reverse=True)
    return notifications

@app.put("/api/notifications/{notification_id}/read", tags=["Bildirishnomalar"])
def mark_notification_read(notification_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Bildirishnomani o'qilgan deb belgilash"""
    lang = get_user_language(dict(request.headers))
    
    notification = NOTIFICATIONS_DB.get(notification_id)
    if not notification or notification["user_id"] != current_user["user_id"]:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("notification_not_found", lang)
        )
    
    notification["is_read"] = True
    NOTIFICATIONS_DB[notification_id] = notification
    return {"message": get_translation("notification_marked_read", lang)}

@app.put("/api/notifications/mark-all-read", tags=["Bildirishnomalar"])
def mark_all_notifications_read(request: Request, current_user: dict = Depends(verify_token)):
    """Barcha bildirishnomalarni o'qilgan deb belgilash"""
    lang = get_user_language(dict(request.headers))
    
    count = 0
    for notification in NOTIFICATIONS_DB.values():
        if notification["user_id"] == current_user["user_id"] and not notification["is_read"]:
            notification["is_read"] = True
            count += 1
    
    return {"message": f"{count} {get_translation('all_notifications_read', lang)}"}

@app.get("/api/notifications/unread-count", tags=["Bildirishnomalar"])
def get_unread_notifications_count(current_user: dict = Depends(verify_token)):
    """O'qilmagan bildirishnomalar soni"""
    count = sum(1 for n in NOTIFICATIONS_DB.values() 
                if n["user_id"] == current_user["user_id"] and not n["is_read"])
    return {"unread_count": count}

# ========== AKTSIYALAR ==========
@app.get("/api/promotions", response_model=List[Promotion], tags=["Aktsiyalar"])
def get_active_promotions():
    """Faol aktsiyalarni olish"""
    now = datetime.now().strftime("%Y-%m-%d")
    active_promotions = []
    
    for promo in PROMOTIONS_DB.values():
        if promo["is_active"] and promo["start_date"] <= now <= promo["end_date"]:
            active_promotions.append(promo)
    
    return active_promotions

@app.get("/api/promotions/all", response_model=List[Promotion], tags=["Aktsiyalar"])
def get_all_promotions(request: Request, current_user: dict = Depends(verify_token)):
    """Barcha aktsiyalar (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    return list(PROMOTIONS_DB.values())

@app.post("/api/promotions", response_model=Promotion, tags=["Aktsiyalar"])
def create_promotion(promotion: Promotion, request: Request, current_user: dict = Depends(verify_token)):
    """Yangi aksiya yaratish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    promo_id = generate_id("promo")
    promotion.id = promo_id
    PROMOTIONS_DB[promo_id] = promotion.dict()
    return promotion

@app.put("/api/promotions/{promo_id}", response_model=Promotion, tags=["Aktsiyalar"])
def update_promotion(promo_id: str, promotion_update: Promotion, request: Request, current_user: dict = Depends(verify_token)):
    """Aksiyani yangilash (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    if promo_id not in PROMOTIONS_DB:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("promotion_not_found", lang)
        )
    
    promotion_update.id = promo_id
    PROMOTIONS_DB[promo_id] = promotion_update.dict()
    return promotion_update

@app.delete("/api/promotions/{promo_id}", tags=["Aktsiyalar"])
def delete_promotion(promo_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Aksiyani o'chirish (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    if promo_id not in PROMOTIONS_DB:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("promotion_not_found", lang)
        )
    
    del PROMOTIONS_DB[promo_id]
    return {"message": get_translation("promotion_deleted", lang)}

@app.post("/api/orders/apply-promo", tags=["Buyurtmalar"])
def apply_promo_code(order_total: int, promo_code: str, request: Request):
    """Promo kod qo'llash"""
    lang = get_user_language(dict(request.headers))
    now = datetime.now().strftime("%Y-%m-%d")
    
    for promo in PROMOTIONS_DB.values():
        if (promo["promo_code"] == promo_code and 
            promo["is_active"] and 
            promo["start_date"] <= now <= promo["end_date"] and
            order_total >= promo["min_order_amount"]):
            
            discount = (order_total * promo["discount_percent"]) // 100
            new_total = order_total - discount
            
            return {
                "valid": True,
                "discount": discount,
                "new_total": new_total,
                "promotion_title": promo["title"]
            }
    
    return {
        "valid": False, 
        "message": get_translation("promo_code_invalid", lang)
    }

# ========== ANALYTICS VA HISOBOTLAR ==========
@app.get("/api/analytics", response_model=Analytics, tags=["Analytics"])
def get_analytics(request: Request, current_user: dict = Depends(verify_token)):
    """Restoran analytics ma'lumotlari (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    # Umumiy buyurtmalar
    total_orders = len(ORDERS_DB)
    
    # Umumiy daromad
    total_revenue = sum(order["total_price"] for order in ORDERS_DB.values() 
                       if order["payment_info"]["status"] == "paid")
    
    # Mashhur ovqatlar
    food_counts = {}
    for order in ORDERS_DB.values():
        for food in order["foods"]:
            food_id = food["id"]
            if food_id not in food_counts:
                food_counts[food_id] = {"name": food["name"], "count": 0, "revenue": 0}
            food_counts[food_id]["count"] += food["count"]
            food_counts[food_id]["revenue"] += food["total_price"]
    
    popular_foods = sorted(food_counts.values(), key=lambda x: x["count"], reverse=True)[:5]
    
    # Kunlik buyurtmalar
    daily_stats = {}
    for order in ORDERS_DB.values():
        date = order["order_time"].split(" ")[0]
        if date not in daily_stats:
            daily_stats[date] = {"date": date, "orders": 0, "revenue": 0}
        daily_stats[date]["orders"] += 1
        if order["payment_info"]["status"] == "paid":
            daily_stats[date]["revenue"] += order["total_price"]
    
    daily_orders = list(daily_stats.values())
    daily_orders.sort(key=lambda x: x["date"])
    
    # Foydalanuvchi statistikasi
    total_users = len([u for u in USERS_DB.values() if u["role"] == "user"])
    active_users = len(set(order["user_number"] for order in ORDERS_DB.values()))
    
    user_statistics = {
        "total_users": total_users,
        "active_users": active_users,
        "conversion_rate": round((active_users / total_users * 100) if total_users > 0 else 0, 2)
    }
    
    return Analytics(
        total_orders=total_orders,
        total_revenue=total_revenue,
        popular_foods=popular_foods,
        daily_orders=daily_orders,
        user_statistics=user_statistics
    )

@app.get("/api/analytics/export", tags=["Analytics"])
def export_analytics(
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    request: Request = None,
    current_user: dict = Depends(verify_token)
):
    """Analytics ma'lumotlarini eksport qilish"""
    lang = get_user_language(dict(request.headers)) if request else "uz"
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    # Buyurtmalarni filtrlash
    filtered_orders = []
    for order in ORDERS_DB.values():
        order_date = order["order_time"].split(" ")[0]
        if start_date and order_date < start_date:
            continue
        if end_date and order_date > end_date:
            continue
        filtered_orders.append(order)
    
    # CSV formatida ma'lumotlar
    csv_data = "Order ID,Date,Customer,Total,Status,Payment Method\n"
    for order in filtered_orders:
        csv_data += f"{order['order_id']},{order['order_time']},{order['user_name']},{order['total_price']},{order['status']},{order['payment_info']['method']}\n"
    
    return {"csv_data": csv_data, "total_orders": len(filtered_orders)}

# ========== INVENTAR BOSHQARUVI ==========
@app.get("/api/inventory", response_model=List[InventoryItem], tags=["Inventar"])
def get_inventory(request: Request, current_user: dict = Depends(verify_token)):
    """Inventar ro'yxatini olish"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    return list(INVENTORY_DB.values())

@app.post("/api/inventory", response_model=InventoryItem, tags=["Inventar"])
def create_inventory_item(item: InventoryItem, request: Request, current_user: dict = Depends(verify_token)):
    """Yangi inventar elementi qo'shish"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    item_id = generate_id("inv")
    item.id = item_id
    item.last_updated = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    INVENTORY_DB[item_id] = item.dict()
    return item

@app.put("/api/inventory/{item_id}", tags=["Inventar"])
def update_inventory(
    item_id: str, 
    update: InventoryUpdate, 
    request: Request,
    current_user: dict = Depends(verify_token)
):
    """Inventar miqdorini yangilash"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    item = INVENTORY_DB.get(item_id)
    if not item:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("inventory_item_not_found", lang)
        )
    
    if update.operation == "add":
        item["quantity"] += update.quantity
    elif update.operation == "subtract":
        item["quantity"] = max(0, item["quantity"] - update.quantity)
    elif update.operation == "set":
        item["quantity"] = update.quantity
    
    item["last_updated"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    INVENTORY_DB[item_id] = item
    
    # Kam qolgan mahsulotlar haqida ogohlantirish
    if item["quantity"] <= item["min_threshold"]:
        create_notification(
            user_id=current_user["user_id"],
            title=get_translation("low_stock_warning", lang),
            message=f"{item['name']} {get_translation('low_stock_warning', lang)} {item['quantity']} {item['unit']}",
            notification_type="system"
        )
    
    return {
        "message": get_translation("inventory_updated", lang), 
        "new_quantity": item["quantity"]
    }

@app.get("/api/inventory/low-stock", tags=["Inventar"])
def get_low_stock_items(request: Request, current_user: dict = Depends(verify_token)):
    """Kam qolgan mahsulotlar ro'yxati"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    low_stock = [item for item in INVENTORY_DB.values() 
                 if item["quantity"] <= item["min_threshold"]]
    
    return low_stock

@app.delete("/api/inventory/{item_id}", tags=["Inventar"])
def delete_inventory_item(item_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Inventar elementini o'chirish"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    if item_id not in INVENTORY_DB:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("inventory_item_not_found", lang)
        )
    
    del INVENTORY_DB[item_id]
    return {"message": get_translation("inventory_item_deleted", lang)}

# ========== XODIMLAR BOSHQARUVI ==========
@app.get("/api/staff", response_model=List[Staff], tags=["Xodimlar"])
def get_staff(request: Request, current_user: dict = Depends(verify_token)):
    """Xodimlar ro'yxati"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    return [staff for staff in STAFF_DB.values() if staff["is_active"]]

@app.post("/api/staff", response_model=Staff, tags=["Xodimlar"])
def create_staff(staff: StaffCreate, request: Request, current_user: dict = Depends(verify_token)):
    """Yangi xodim qo'shish"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    staff_id = generate_id("staff")
    new_staff = Staff(
        id=staff_id,
        full_name=staff.full_name,
        position=staff.position,
        phone=staff.phone,
        email=staff.email,
        hire_date=datetime.now().strftime("%Y-%m-%d"),
        salary=staff.salary
    )
    
    STAFF_DB[staff_id] = new_staff.dict()
    return new_staff

@app.put("/api/staff/{staff_id}", response_model=Staff, tags=["Xodimlar"])
def update_staff(staff_id: str, staff_update: StaffCreate, request: Request, current_user: dict = Depends(verify_token)):
    """Xodim ma'lumotlarini yangilash"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    staff = STAFF_DB.get(staff_id)
    if not staff:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("staff_not_found", lang)
        )
    
    # Ma'lumotlarni yangilash
    update_data = staff_update.dict()
    for field, value in update_data.items():
        if value is not None:
            staff[field] = value
    
    STAFF_DB[staff_id] = staff
    return staff

@app.delete("/api/staff/{staff_id}", tags=["Xodimlar"])
def delete_staff(staff_id: str, request: Request, current_user: dict = Depends(verify_token)):
    """Xodimni o'chirish"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    staff = STAFF_DB.get(staff_id)
    if not staff:
        raise HTTPException(
            status_code=404, 
            detail=get_translation("staff_not_found", lang)
        )
    
    staff["is_active"] = False
    STAFF_DB[staff_id] = staff
    return {"message": get_translation("staff_deleted", lang)}

# ========== MIJOZLAR XIZMATI ==========
@app.get("/api/tickets", response_model=List[SupportTicket], tags=["Mijozlar xizmati"])
def get_tickets(request: Request, current_user: dict = Depends(verify_token)):
    """Support ticketlar ro'yxati"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] == "admin":
        # Admin barcha ticketlarni ko'radi
        tickets = list(TICKETS_DB.values())
    else:
        # Foydalanuvchi faqat o'z ticketlarini ko'radi
        tickets = [t for t in TICKETS_DB.values() if t["user_id"] == current_user["user_id"]]
    
    tickets.sort(key=lambda x: x["created_at"], reverse=True)
    return tickets

@app.post("/api/tickets", response_model=SupportTicket, tags=["Mijozlar xizmati"])
def create_ticket(ticket: TicketCreate, request: Request, current_user: dict = Depends(verify_token)):
    """Yangi support ticket yaratish"""
    lang = get_user_language(dict(request.headers))
    
    ticket_id = generate_id("ticket")
    new_ticket = SupportTicket(
        id=ticket_id,
        user_id=current_user["user_id"],
        subject=ticket.subject,
        message=ticket.message,
        created_at=datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    )
    
    TICKETS_DB[ticket_id] = new_ticket.dict()
    return new_ticket

@app.put("/api/tickets/{ticket_id}/status", tags=["Mijozlar xizmati"])
def update_ticket_status(ticket_id: str, new_status: str, request: Request, current_user: dict = Depends(verify_token)):
    """Ticket holatini yangilash (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    ticket = TICKETS_DB.get(ticket_id)
    if not ticket:
        raise HTTPException(status_code=404, detail="Ticket topilmadi")
    
    ticket["status"] = new_status
    if new_status in ["resolved", "closed"]:
        ticket["resolved_at"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    TICKETS_DB[ticket_id] = ticket
    return {"message": "Ticket holati yangilandi"}

# ========== SOZLAMALAR ==========
@app.get("/api/settings/restaurant", tags=["Sozlamalar"])
def get_restaurant_settings():
    """Restoran sozlamalari"""
    return SETTINGS["restaurant"]

@app.put("/api/settings/restaurant", tags=["Sozlamalar"])
def update_restaurant_settings(settings: RestaurantSettings, request: Request, current_user: dict = Depends(verify_token)):
    """Restoran sozlamalarini yangilash (admin uchun)"""
    lang = get_user_language(dict(request.headers))
    
    if current_user["role"] != "admin":
        raise HTTPException(
            status_code=403, 
            detail=get_translation("forbidden", lang)
        )
    
    SETTINGS["restaurant"] = settings.dict()
    return {"message": get_translation("success", lang)}

# ========== TEST ENDPOINTLARI ==========
@app.get("/api/translations/{lang}", tags=["Test"])
def get_translations(lang: str):
    """Til tarjimalarini ko'rish (test uchun)"""
    if lang not in TRANSLATIONS:
        raise HTTPException(status_code=404, detail="Til topilmadi")
    
    return TRANSLATIONS[lang]

@app.get("/api/test/multilingual-foods", tags=["Test"])
def test_multilingual_foods(lang: str = "uz"):
    """Ko'p tilli ovqatlarni test qilish"""
    return get_all_localized_foods(lang)

@app.get("/api/test/food/{food_id}", tags=["Test"])
def test_single_food(food_id: str, lang: str = "uz"):
    """Bitta ovqatni test qilish"""
    food = get_localized_food(food_id, lang)
    if not food:
        raise HTTPException(status_code=404, detail=f"Food {food_id} not found")
    
    return {
        "food_id": food_id,
        "language": lang,
        "food_data": food,
        "source": "multilingual_db" if food_id in MULTILINGUAL_FOODS_DB else "foods_db"
    }

@app.get("/api/test/compare-foods", tags=["Test"])
def compare_foods_by_language():
    """Ovqatlarni barcha tillarda solishtirish"""
    comparison = {}
    
    # Ko'p tilli ovqatlarni test qilish
    for food_id in list(MULTILINGUAL_FOODS_DB.keys())[:3]:  # faqat birinchi 3 tani
        comparison[food_id] = {}
        for lang in ["uz", "ru", "en"]:
            food = get_localized_food(food_id, lang)
            if food:
                comparison[food_id][lang] = {
                    "name": food["name"],
                    "description": food["description"],
                    "ingredients": food["ingredients"]
                }
    
    return comparison

@app.post("/api/test/create-order", tags=["Test"])
async def test_create_multilingual_order(request: Request, current_user: dict = Depends(verify_token)):
    """Ko'p tilli buyurtma yaratishni test qilish"""
    lang = get_user_language(dict(request.headers))
    
    # Test buyurtmasi
    test_order_request = OrderRequest(
        food_ids=[
            {"amur_1": 2},  # Moloti
            {"amur_3": 1},  # Osh
            {"amur_9": 3}   # Somsa
        ],
        to_give={"delivery": "Test manzil, Toshkent"},
        payment_method=PaymentMethod.CASH,
        special_instructions="Test buyurtmasi - ko'p tilli"
    )
    
    # Test foydalanuvchisi ma'lumotlari
    test_user = {
        "number": current_user["number"],
        "user_id": current_user["user_id"],
        "role": current_user["role"]
    }
    
    # Ovqatlarni ko'p tilli formatda olish
    test_foods = []
    for food_dict in test_order_request.food_ids:
        for food_id, count in food_dict.items():
            food = get_localized_food(food_id, lang)
            if food:
                test_foods.append({
                    "id": food_id,
                    "name": food["name"],
                    "description": food["description"],
                    "category": food.get("category_name", food["category"]),
                    "count": count,
                    "price": food["price"],
                    "total": food["price"] * count
                })
    
    return {
        "message": f"Test buyurtmasi ({lang} tilida)",
        "language": lang,
        "test_foods": test_foods,
        "user": test_user,
        "note": "Bu test endpoint. Haqiqiy buyurtma yaratish uchun POST /api/orders ishlatiladi"
    }

# ========== ASOSIY ENDPOINT ==========
@app.get("/", tags=["Asosiy"])
def root():
    """API haqida ma'lumot"""
    return {
        "message": "Restaurant API - Ko'p tilli qo'llab-quvvatlash bilan",
        "version": "2.1.0",
        "supported_languages": ["uz", "ru", "en"],
        "endpoints": {
            "foods": "/api/foods/localized",
            "categories": "/api/categories", 
            "search": "/api/search",
            "language_settings": "/api/settings/language"
        }
    }

# ========== SERVER ISHGA TUSHIRISH ==========
if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)