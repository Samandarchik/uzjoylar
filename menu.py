from fastapi import FastAPI, HTTPException, Depends, status, File, UploadFile, Form, BackgroundTasks
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

# FastAPI ilovasini yaratamiz
app = FastAPI(
    title="Restaurant API",
    description="Restoran uchun ovqatlar boshqaruvi API",
    version="2.0.0"
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
TELEGRAM_BOT_TOKEN = "8117502669:AAH-UAP6Vd9oihS3rXyyrEov0Q34OdeYnZ4"  
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
    tg_id: Optional[int] = None  # Telegram ID qo'shildi

class LoginResponse(BaseModel):
    token: str
    role: str
    user_id: str

class User(BaseModel):
    id: str
    number: str
    full_name: str
    email: Optional[str] = None
    role: str
    created_at: str
    is_active: bool = True
    tg_id: Optional[int] = None  # Telegram ID qo'shildi

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
    preparation_time: Optional[int] = 15  # daqiqalarda

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
    food_ids: List[dict]  # [{"amur_1": 4}, {"amur_3": 3}]
    to_give: dict  # delivery, own_withdrawal, atTheRestaurant
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
    estimated_time: Optional[int] = None  # daqiqalarda
    delivered_at: Optional[str] = None

class Notification(BaseModel):
    id: str
    user_id: str
    title: str
    message: str
    is_read: bool = False
    created_at: str
    type: str  # order, promotion, system

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

# INVENTAR BOSHQARUVI modellari
class InventoryItem(BaseModel):
    id: str
    name: str
    quantity: int
    unit: str  # kg, dona, litr
    min_threshold: int
    supplier: Optional[str] = None
    last_updated: str

class InventoryUpdate(BaseModel):
    quantity: int
    operation: str  # add, subtract, set

# XODIMLAR BOSHQARUVI modellari
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

# MIJOZLAR XIZMATI modellari
class SupportTicket(BaseModel):
    id: str
    user_id: str
    subject: str
    message: str
    status: str = "open"  # open, in_progress, resolved, closed
    created_at: str
    resolved_at: Optional[str] = None

class TicketCreate(BaseModel):
    subject: str
    message: str

# SOZLAMALAR modellari
class RestaurantSettings(BaseModel):
    name: str
    address: str
    phone: str
    email: str
    working_hours: dict
    delivery_fee: int
    min_order_amount: int
    max_delivery_distance: int  # km

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
        "tg_id": 1713329317  # Admin telegram ID
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
        "tg_id": None
    }
}

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
    "amur_4": {
        "id": "amur_4",
        "name": "Lag'mon",
        "category": "Milliy taomlar",
        "price": 12000,
        "description": "An'anaviy o'zbek lag'moni, sabzavat bilan",
        "isThere": True,
        "imageUrl": "https://example.com/lagmon.jpg",
        "ingredients": ["Makaron", "Mol go'shti", "Sabzavotlar"],
        "allergens": ["Gluten"],
        "rating": 4.3,
        "review_count": 18,
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

# Email sozlamalari (ixtiyoriy)
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

def format_order_for_telegram(order: dict) -> str:
    """Buyurtmani Telegram uchun formatlash"""
    foods_list = ""
    for food in order["foods"]:
        foods_list += f"• {food['name']} x{food['count']} = {food['total_price']:,} so'm\n"
    
    delivery_info = ""
    if order["delivery_type"] == "delivery":
        delivery_info = f"📍 Yetkazib berish: {order['delivery_info']['location']}"
    elif order["delivery_type"] == "own_withdrawal":
        delivery_info = f"🏃‍♂️ O'zi olib ketish: {order['delivery_info']['pickup_code']}"
    elif order["delivery_type"] == "atTheRestaurant":
        delivery_info = f"🍽 Restoranda: {order['delivery_info']['table_name']}"
    
    message = f"""
🍽 <b>Yangi buyurtma!</b>

📋 <b>Buyurtma ID:</b> {order['order_id']}
👤 <b>Mijoz:</b> {order['user_name']}
📞 <b>Telefon:</b> {order['user_number']}
📅 <b>Vaqt:</b> {order['order_time']}

<b>Buyurtma tarkibi:</b>
{foods_list}
💰 <b>Umumiy summa:</b> {order['total_price']:,} so'm

{delivery_info}
💳 <b>To'lov usuli:</b> {order['payment_info']['method']}
⏱ <b>Tayyorlash vaqti:</b> {order.get('estimated_time', 'Nomalum')} daqiqa

{f"📝 <b>Qo'shimcha:</b> {order['special_instructions']}" if order.get('special_instructions') else ""}
"""
    return message

def format_notification_for_telegram(title: str, message: str, order_id: str = None) -> str:
    """Bildirishnomani Telegram uchun formatlash"""
    if order_id:
        return f"🔔 <b>{title}</b>\n\n{message}\n\n📋 Buyurtma ID: {order_id}"
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
    """Email yuborish (ixtiyoriy)"""
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
    status_messages = {
        "confirmed": "Buyurtmangiz tasdiqlandi!",
        "preparing": "Buyurtmangiz tayyorlanmoqda...",
        "ready": "Buyurtmangiz tayyor!",
        "delivered": "Buyurtmangiz yetkazildi!",
        "cancelled": "Buyurtmangiz bekor qilindi."
    }
    
    if status in status_messages:
        # Lokal bildirishnoma yaratish
        create_notification(
            user_id=user_id,
            title=f"Buyurtma #{order_id}",
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
                f"Buyurtma #{order_id}",
                status_messages[status],
                order_id
            )
            await send_telegram_message(user["tg_id"], telegram_message)

# API endpointlari

# ========== AUTENTIFIKATSIYA ==========
@app.post("/api/register", response_model=LoginResponse, tags=["Autentifikatsiya"])
def register(request: RegisterRequest):
    """Yangi foydalanuvchi ro'yxatdan o'tkazish"""
    if request.number in USERS_DB:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Bu telefon raqami allaqachon ro'yxatdan o'tgan"
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
        "tg_id": request.tg_id  # Telegram ID qo'shildi
    }
    
    USERS_DB[request.number] = new_user
    
    token = create_access_token({
        "sub": request.number, 
        "role": "user",
        "user_id": user_id
    })
    
    return LoginResponse(token=token, role="user", user_id=user_id)

@app.post("/api/login", response_model=LoginResponse, tags=["Autentifikatsiya"])
def login(request: LoginRequest):
    """Foydalanuvchi tizimga kirishi"""
    user = USERS_DB.get(request.number)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Telefon raqami yoki parol noto'g'ri"
        )
    
    password_hash = hashlib.md5(request.password.encode()).hexdigest()
    if user["password"] != password_hash:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Telefon raqami yoki parol noto'g'ri"
        )
    
    token = create_access_token({
        "sub": request.number, 
        "role": user["role"],
        "user_id": user["id"]
    })
    
    return LoginResponse(token=token, role=user["role"], user_id=user["id"])

@app.get("/api/profile", response_model=User, tags=["Foydalanuvchi"])
def get_profile(current_user: dict = Depends(verify_token)):
    """Foydalanuvchi profilini olish"""
    user = USERS_DB.get(current_user["number"])
    if not user:
        raise HTTPException(status_code=404, detail="Foydalanuvchi topilmadi")
    
    user_copy = user.copy()
    user_copy.pop("password", None)
    return user_copy

# ========== OVQATLAR ==========
@app.get("/api/foods", response_model=List[Food], tags=["Ovqatlar"])
def get_all_foods(category: Optional[str] = None, search: Optional[str] = None):
    """Barcha ovqatlar ro'yxatini olish"""
    foods = list(FOODS_DB.values())
    
    if category:
        foods = [f for f in foods if f["category"].lower() == category.lower()]
    
    if search:
        search_lower = search.lower()
        foods = [f for f in foods if search_lower in f["name"].lower() or search_lower in f["description"].lower()]
    
    return foods

@app.get("/api/foods/popular", response_model=List[Food], tags=["Ovqatlar"])
def get_popular_foods(limit: int = 5):
    """Mashhur ovqatlarni olish"""
    foods = list(FOODS_DB.values())
    foods.sort(key=lambda x: (x.get("rating", 0), x.get("review_count", 0)), reverse=True)
    return foods[:limit]

@app.get("/api/foods/{food_id}", response_model=Food, tags=["Ovqatlar"])
def get_food(food_id: str):
    """Bitta ovqat ma'lumotlari"""
    food = FOODS_DB.get(food_id)
    if not food:
        raise HTTPException(status_code=404, detail="Ovqat topilmadi")
    return food

@app.post("/api/foods", response_model=Food, tags=["Ovqatlar"])
def create_food(food: FoodCreate, current_user: dict = Depends(verify_token)):
    """Yangi ovqat qo'shish (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin ovqat qo'sha oladi")
    
    food_id = generate_id("food")
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
        preparation_time=food.preparation_time
    )
    
    FOODS_DB[food_id] = new_food.dict()
    return new_food

@app.put("/api/foods/{food_id}", response_model=Food, tags=["Ovqatlar"])
def update_food(food_id: str, food_update: FoodUpdate, current_user: dict = Depends(verify_token)):
    """Ovqat ma'lumotlarini yangilash (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin ovqat ma'lumotlarini yangilashi mumkin")
    
    food = FOODS_DB.get(food_id)
    if not food:
        raise HTTPException(status_code=404, detail="Ovqat topilmadi")
    
    # Ma'lumotlarni yangilash
    update_data = food_update.dict(exclude_unset=True)
    for field, value in update_data.items():
        food[field] = value
    
    FOODS_DB[food_id] = food
    return food

@app.delete("/api/foods/{food_id}", tags=["Ovqatlar"])
def delete_food(food_id: str, current_user: dict = Depends(verify_token)):
    """Ovqatni o'chirish (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin ovqat o'chira oladi")
    
    if food_id not in FOODS_DB:
        raise HTTPException(status_code=404, detail="Ovqat topilmadi")
    
    del FOODS_DB[food_id]
    return {"message": "Ovqat o'chirildi"}

@app.post("/api/foods/upload-image", tags=["Ovqatlar"])
def upload_food_image(file: UploadFile = File(...), current_user: dict = Depends(verify_token)):
    """Ovqat rasmi yuklash"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin rasm yuklay oladi")
    
    try:
        image_url = save_uploaded_file(file)
        return {"imageUrl": image_url, "message": "Rasm muvaffaqiyatli yuklandi"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ========== REVIEWS ==========
@app.post("/api/reviews", response_model=Review, tags=["Sharhlar"])
def create_review(review: ReviewCreate, current_user: dict = Depends(verify_token)):
    """Ovqat uchun sharh qoldirish"""
    if review.food_id not in FOODS_DB:
        raise HTTPException(status_code=404, detail="Ovqat topilmadi")
    
    # Bir foydalanuvchi bir ovqat uchun faqat bitta sharh qoldira oladi
    existing_review = None
    for r in REVIEWS_DB.values():
        if r["user_id"] == current_user["user_id"] and r["food_id"] == review.food_id:
            existing_review = r
            break
    
    if existing_review:
        raise HTTPException(status_code=400, detail="Siz bu ovqat uchun allaqachon sharh qoldirgan ekansiz")
    
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
    FOODS_DB[review.food_id]["rating"] = rating
    FOODS_DB[review.food_id]["review_count"] = count
    
    return new_review

@app.get("/api/foods/{food_id}/reviews", response_model=List[Review], tags=["Sharhlar"])
def get_food_reviews(food_id: str):
    """Ovqat sharhlari"""
    reviews = [r for r in REVIEWS_DB.values() if r["food_id"] == food_id]
    reviews.sort(key=lambda x: x["created_at"], reverse=True)
    return reviews

@app.get("/api/reviews/{review_id}", response_model=Review, tags=["Sharhlar"])
def get_review(review_id: str):
    """Bitta sharh ma'lumotlari"""
    review = REVIEWS_DB.get(review_id)
    if not review:
        raise HTTPException(status_code=404, detail="Sharh topilmadi")
    return review

@app.delete("/api/reviews/{review_id}", tags=["Sharhlar"])
def delete_review(review_id: str, current_user: dict = Depends(verify_token)):
    """Sharhni o'chirish"""
    review = REVIEWS_DB.get(review_id)
    if not review:
        raise HTTPException(status_code=404, detail="Sharh topilmadi")
    
    # Faqat sharh egasi yoki admin o'chira oladi
    if review["user_id"] != current_user["user_id"] and current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Bu sharhni o'chirish huquqingiz yo'q")
    
    food_id = review["food_id"]
    del REVIEWS_DB[review_id]
    
    # Ovqat reytingini qayta hisoblash
    rating, count = calculate_food_rating(food_id)
    FOODS_DB[food_id]["rating"] = rating
    FOODS_DB[food_id]["review_count"] = count
    
    return {"message": "Sharh o'chirildi"}

# ========== BUYURTMALAR (kengaytirilgan) ==========
@app.post("/api/orders", response_model=Order, tags=["Buyurtmalar"])
async def create_order(
    order_request: OrderRequest, 
    background_tasks: BackgroundTasks,
    current_user: dict = Depends(verify_token)
):
    """Yangi buyurtma berish (kengaytirilgan)"""
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
            raise HTTPException(status_code=400, detail=f"Ovqat miqdori 0 dan katta bo'lishi kerak: {food_id}")
        
        food = FOODS_DB.get(food_id)
        if not food or not food["isThere"]:
            raise HTTPException(status_code=400, detail=f"Ovqat mavjud emas: {food_id}")
        
        food_total_price = food["price"] * count
        prep_time = food.get("preparation_time", 15)
        total_prep_time = max(total_prep_time, prep_time)
        
        ordered_food = OrderFood(
            id=food["id"],
            name=food["name"],
            category=food["category"],
            price=food["price"],
            description=food["description"],
            imageUrl=food["imageUrl"],
            count=count,
            total_price=food_total_price
        )
        ordered_foods.append(ordered_food)
        total_price += food_total_price
    
    # Yetkazib berish ma'lumotlari
    delivery_type = ""
    delivery_info = {}
    delivery_info_text = ""
    
    if "delivery" in order_request.to_give:
        delivery_type = "delivery"
        delivery_info = {
            "type": "delivery",
            "location": order_request.to_give["delivery"]
        }
        delivery_info_text = f"📍 <b>Yetkazib berish:</b> {order_request.to_give['delivery']}"
        total_prep_time += 20  # yetkazib berish vaqti
    elif "own_withdrawal" in order_request.to_give:
        delivery_type = "own_withdrawal"
        delivery_info = {
            "type": "own_withdrawal",
            "pickup_code": order_request.to_give["own_withdrawal"]
        }
        delivery_info_text = f"🏃‍♂️ <b>O'zi olib ketish:</b> {order_request.to_give['own_withdrawal']}"
    elif "atTheRestaurant" in order_request.to_give:
        delivery_type = "atTheRestaurant"
        table_id = order_request.to_give["atTheRestaurant"]
        table_name = get_table_name_by_id(table_id)
        delivery_info = {
            "type": "atTheRestaurant",
            "table_id": table_id,
            "table_name": table_name
        }
        delivery_info_text = f"🍽 <b>Restoranda:</b> {table_name}"
    
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
        foods=ordered_foods,
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
    telegram_message = format_order_for_telegram(order_dict)
    
    # Adminlarga yuborish
    for admin_user in USERS_DB.values():
        if admin_user["role"] == "admin" and admin_user.get("tg_id"):
            background_tasks.add_task(
                send_telegram_message,
                admin_user["tg_id"],
                telegram_message
            )
    
    # Foydalanuvchiga tasdiqlash xabari
    if user and user.get("tg_id"):
        # Buyurtma tarkibini formatlash
        foods_detail = ""
        for food in ordered_foods:
            foods_detail += f"• {food.name} x{food.count} = {food.total_price:,} so'm\n"
        
        user_message = f"""
✅ <b>Buyurtmangiz qabul qilindi!</b>

📋 <b>Buyurtma ID:</b> {order_id}
📅 <b>Vaqt:</b> {order_time}

<b>Buyurtma tarkibi:</b>
{foods_detail}
💰 <b>Umumiy summa:</b> {total_price:,} so'm
⏱ <b>Tayyorlash vaqti:</b> {total_prep_time} daqiqa

{delivery_info_text}

Buyurtmangiz holati haqida xabar berib turamiz!
        """
        background_tasks.add_task(
            send_telegram_message,
            user["tg_id"],
            user_message
        )
        
        # Debug uchun log qo'shish
        logger.info(f"Foydalanuvchiga xabar yuborildi: {user['tg_id']}")
    else:
        logger.warning(f"Foydalanuvchi telegram ID topilmadi: {current_user['number']}")
    
    # Telegram guruhga ham yuborish
    background_tasks.add_task(
        send_telegram_message,
        TELEGRAM_GROUP_ID,
        telegram_message
    )
    
    return new_order

@app.get("/api/orders", response_model=List[Order], tags=["Buyurtmalar"])
def get_orders(current_user: dict = Depends(verify_token)):
    """Buyurtmalar ro'yxati"""
    if current_user["role"] == "admin":
        # Admin barcha buyurtmalarni ko'radi
        orders = list(ORDERS_DB.values())
    else:
        # Oddiy foydalanuvchi faqat o'z buyurtmalarini ko'radi
        orders = [order for order in ORDERS_DB.values() if order["user_number"] == current_user["number"]]
    
    # Vaqt bo'yicha tartiblash (eng yangi birinchi)
    orders.sort(key=lambda x: x["order_time"], reverse=True)
    return orders

@app.get("/api/orders/{order_id}", response_model=Order, tags=["Buyurtmalar"])
def get_order(order_id: str, current_user: dict = Depends(verify_token)):
    """Bitta buyurtma ma'lumotlari"""
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Buyurtma topilmadi")
    
    # Foydalanuvchi faqat o'z buyurtmasini ko'ra oladi
    if current_user["role"] != "admin" and order["user_number"] != current_user["number"]:
        raise HTTPException(status_code=403, detail="Bu buyurtmani ko'rish huquqingiz yo'q")
    
    return order

@app.put("/api/orders/{order_id}/status", tags=["Buyurtmalar"])
async def update_order_status(
    order_id: str, 
    new_status: OrderStatus,
    background_tasks: BackgroundTasks,
    current_user: dict = Depends(verify_token)
):
    """Buyurtma holatini yangilash"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin buyurtma holatini o'zgartira oladi")
    
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Buyurtma topilmadi")
    
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
    
    return {"message": f"Buyurtma holati '{new_status.value}' ga o'zgartirildi"}

@app.delete("/api/orders/{order_id}", tags=["Buyurtmalar"])
def cancel_order(order_id: str, current_user: dict = Depends(verify_token)):
    """Buyurtmani bekor qilish"""
    order = ORDERS_DB.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail="Buyurtma topilmadi")
    
    # Faqat buyurtma egasi yoki admin bekor qila oladi
    if order["user_number"] != current_user["number"] and current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Bu buyurtmani bekor qilish huquqingiz yo'q")
    
    # Faqat pending yoki confirmed holatdagi buyurtmalarni bekor qilish mumkin
    if order["status"] not in ["pending", "confirmed"]:
        raise HTTPException(status_code=400, detail="Bu buyurtmani bekor qilib bo'lmaydi")
    
    order["status"] = OrderStatus.CANCELLED.value
    ORDERS_DB[order_id] = order
    
    return {"message": "Buyurtma bekor qilindi"}

# ========== BILDIRISHNOMALAR ==========
@app.get("/api/notifications", response_model=List[Notification], tags=["Bildirishnomalar"])
def get_notifications(current_user: dict = Depends(verify_token)):
    """Foydalanuvchi bildirishnomalarini olish"""
    notifications = [n for n in NOTIFICATIONS_DB.values() if n["user_id"] == current_user["user_id"]]
    notifications.sort(key=lambda x: x["created_at"], reverse=True)
    return notifications

@app.put("/api/notifications/{notification_id}/read", tags=["Bildirishnomalar"])
def mark_notification_read(notification_id: str, current_user: dict = Depends(verify_token)):
    """Bildirishnomani o'qilgan deb belgilash"""
    notification = NOTIFICATIONS_DB.get(notification_id)
    if not notification or notification["user_id"] != current_user["user_id"]:
        raise HTTPException(status_code=404, detail="Bildirishnoma topilmadi")
    
    notification["is_read"] = True
    NOTIFICATIONS_DB[notification_id] = notification
    return {"message": "Bildirishnoma o'qilgan deb belgilandi"}

@app.put("/api/notifications/mark-all-read", tags=["Bildirishnomalar"])
def mark_all_notifications_read(current_user: dict = Depends(verify_token)):
    """Barcha bildirishnomalarni o'qilgan deb belgilash"""
    count = 0
    for notification in NOTIFICATIONS_DB.values():
        if notification["user_id"] == current_user["user_id"] and not notification["is_read"]:
            notification["is_read"] = True
            count += 1
    
    return {"message": f"{count} ta bildirishnoma o'qilgan deb belgilandi"}

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
def get_all_promotions(current_user: dict = Depends(verify_token)):
    """Barcha aktsiyalar (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin barcha aktsiyalarni ko'ra oladi")
    
    return list(PROMOTIONS_DB.values())

@app.post("/api/promotions", response_model=Promotion, tags=["Aktsiyalar"])
def create_promotion(promotion: Promotion, current_user: dict = Depends(verify_token)):
    """Yangi aksiya yaratish (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin aksiya yarata oladi")
    
    promo_id = generate_id("promo")
    promotion.id = promo_id
    PROMOTIONS_DB[promo_id] = promotion.dict()
    return promotion

@app.put("/api/promotions/{promo_id}", response_model=Promotion, tags=["Aktsiyalar"])
def update_promotion(promo_id: str, promotion_update: Promotion, current_user: dict = Depends(verify_token)):
    """Aksiyani yangilash (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin aksiya yangilashi mumkin")
    
    if promo_id not in PROMOTIONS_DB:
        raise HTTPException(status_code=404, detail="Aksiya topilmadi")
    
    promotion_update.id = promo_id
    PROMOTIONS_DB[promo_id] = promotion_update.dict()
    return promotion_update

@app.delete("/api/promotions/{promo_id}", tags=["Aktsiyalar"])
def delete_promotion(promo_id: str, current_user: dict = Depends(verify_token)):
    """Aksiyani o'chirish (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin aksiya o'chira oladi")
    
    if promo_id not in PROMOTIONS_DB:
        raise HTTPException(status_code=404, detail="Aksiya topilmadi")
    
    del PROMOTIONS_DB[promo_id]
    return {"message": "Aksiya o'chirildi"}

@app.post("/api/orders/apply-promo", tags=["Buyurtmalar"])
def apply_promo_code(order_total: int, promo_code: str):
    """Promo kod qo'llash"""
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
    
    return {"valid": False, "message": "Promo kod yaroqsiz yoki muddati tugagan"}

# ========== ANALYTICS VA HISOBOTLAR ==========
@app.get("/api/analytics", response_model=Analytics, tags=["Analytics"])
def get_analytics(current_user: dict = Depends(verify_token)):
    """Restoran analytics ma'lumotlari (admin uchun)"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin analytics ko'ra oladi")
    
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
    current_user: dict = Depends(verify_token)
):
    """Analytics ma'lumotlarini eksport qilish"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin eksport qila oladi")
    
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
def get_inventory(current_user: dict = Depends(verify_token)):
    """Inventar ro'yxatini olish"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin inventarni ko'ra oladi")
    
    return list(INVENTORY_DB.values())

@app.post("/api/inventory", response_model=InventoryItem, tags=["Inventar"])
def create_inventory_item(item: InventoryItem, current_user: dict = Depends(verify_token)):
    """Yangi inventar elementi qo'shish"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin inventar qo'sha oladi")
    
    item_id = generate_id("inv")
    item.id = item_id
    item.last_updated = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    INVENTORY_DB[item_id] = item.dict()
    return item

@app.put("/api/inventory/{item_id}", tags=["Inventar"])
def update_inventory(
    item_id: str, 
    update: InventoryUpdate, 
    current_user: dict = Depends(verify_token)
):
    """Inventar miqdorini yangilash"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin inventarni yangilashi mumkin")
    
    item = INVENTORY_DB.get(item_id)
    if not item:
        raise HTTPException(status_code=404, detail="Inventar elementi topilmadi")
    
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
            title="Inventar ogohlantirishsi",
            message=f"{item['name']} tugab qolmoqda! Qolgan: {item['quantity']} {item['unit']}",
            notification_type="system"
        )
    
    return {"message": "Inventar yangilandi", "new_quantity": item["quantity"]}

@app.get("/api/inventory/low-stock", tags=["Inventar"])
def get_low_stock_items(current_user: dict = Depends(verify_token)):
    """Kam qolgan mahsulotlar ro'yxati"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin ko'ra oladi")
    
    low_stock = [item for item in INVENTORY_DB.values() 
                 if item["quantity"] <= item["min_threshold"]]
    
    return low_stock

@app.delete("/api/inventory/{item_id}", tags=["Inventar"])
def delete_inventory_item(item_id: str, current_user: dict = Depends(verify_token)):
    """Inventar elementini o'chirish"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin inventar o'chira oladi")
    
    if item_id not in INVENTORY_DB:
        raise HTTPException(status_code=404, detail="Inventar elementi topilmadi")
    
    del INVENTORY_DB[item_id]
    return {"message": "Inventar elementi o'chirildi"}

# ========== XODIMLAR BOSHQARUVI ==========
@app.get("/api/staff", response_model=List[Staff], tags=["Xodimlar"])
def get_staff(current_user: dict = Depends(verify_token)):
    """Xodimlar ro'yxati"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin xodimlarni ko'ra oladi")
    
    return [staff for staff in STAFF_DB.values() if staff["is_active"]]

@app.post("/api/staff", response_model=Staff, tags=["Xodimlar"])
def create_staff(staff: StaffCreate, current_user: dict = Depends(verify_token)):
    """Yangi xodim qo'shish"""
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Faqat admin xodim qo'sha oladi")
    
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