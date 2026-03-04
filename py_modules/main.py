import MetaTrader5 as mt5
import pandas as pd
import time
import requests
import ujson
from datetime import datetime, timedelta
from ta.trend import EMAIndicator
from ta.momentum import RSIIndicator
from ta.volatility import AverageTrueRange

# ==============================
# SETTINGS ($7 SAFE MODE)
# ==============================
symbol = "XAUUSD"
timeframe = mt5.TIMEFRAME_M1

fixed_lot = 0.01
atr_period = 14
atr_sl_multiplier = 1.0
atr_tp_multiplier = 0.7

max_spread_points = 35
spread_multiplier = 2.0

max_trades_per_hour = 1
max_daily_loss_percent = 2

news_pause_before = 30
news_pause_after = 30

# MAGIC NUMBERS & SYNC
PYTHON_MAGIC = 777001
EA_MAGIC_NUMBERS = {
    123456: "Dark Venus",
    223344: "ExMachina",
    998877: "Gold Zone"
}

trade_log = []
spread_history = []
logged_tickets = set() # Keeps track of what we've sent to Go

# ==============================
# INITIALIZE
# ==============================
if not mt5.initialize():
    print("MT5 initialization failed")
    quit()

# Optimize: Keep symbol in RAM for zero-latency
mt5.symbol_select(symbol, True)

account_info = mt5.account_info()
daily_start_balance = account_info.balance

print(f"Running XAUUSD $7 Safe Scalper on Windows...")
print(f"Monitoring EAs: {list(EA_MAGIC_NUMBERS.values())}")

# ==============================
# NEW: GO DATABASE LOGGING
# ==============================
def log_to_go(order_type, profit, magic, comment=""):
    source = EA_MAGIC_NUMBERS.get(magic, "Python_Bot")
    url = "http://localhost:8080/api/v1/trades/log"
    payload = {
        "user_id": "barbosa_01",
        "symbol": symbol,
        "operation": f"{source}_{order_type}".upper(),
        "volume": fixed_lot,
        "profit": profit,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }
    try:
        requests.post(url, data=ujson.dumps(payload), timeout=0.5)
    except Exception as e:
        print(f"Logger Error: {e}")

# ==============================
# NEW: L2 ORDER BOOK FILTER
# ==============================
def order_book_filter(signal):
    mt5.market_book_add(symbol)
    book = mt5.market_book_get(symbol)
    mt5.market_book_release(symbol)
    
    if not book: return False
        
    buy_vol = sum(item.volume_real for item in book if item.type == mt5.BOOK_TYPE_BUY)
    sell_vol = sum(item.volume_real for item in book if item.type == mt5.BOOK_TYPE_SELL)
    
    if buy_vol == 0 or sell_vol == 0: return False
    
    # Block if the opposite side has 3x more liquidity (Price Wall)
    if signal == "buy" and (sell_vol / buy_vol) > 3.0:
        print(f"Blocked: Sell Wall ({sell_vol} vs {buy_vol})")
        return True
    if signal == "sell" and (buy_vol / sell_vol) > 3.0:
        print(f"Blocked: Buy Wall ({buy_vol} vs {sell_vol})")
        return True
    return False

# ==============================
# REFINED FUNCTIONS
# ==============================
def get_data():
    rates = mt5.copy_rates_from_pos(symbol, timeframe, 0, 300)
    df = pd.DataFrame(rates)
    df['ema20'] = EMAIndicator(df['close'], 20).ema_indicator()
    df['ema50'] = EMAIndicator(df['close'], 50).ema_indicator()
    df['rsi'] = RSIIndicator(df['close'], 14).rsi()
    df['atr'] = AverageTrueRange(df['high'], df['low'], df['close'], atr_period).average_true_range()
    return df

def session_filter():
    hour = datetime.utcnow().hour
    return (7 <= hour <= 11) or (13 <= hour <= 17)

def news_filter():
    now = datetime.utcnow()
    future = now + timedelta(minutes=news_pause_before)
    past = now - timedelta(minutes=news_pause_after)
    events = mt5.calendar_events()
    if events is None: return False
    for event in events:
        event_time = datetime.fromtimestamp(event.time)
        if event.importance == 3 and event.currency == "USD" and (past <= event_time <= future):
            print("Blocked: High Impact USD News")
            return True
    return False

def spread_filter():
    tick = mt5.symbol_info_tick(symbol)
    info = mt5.symbol_info(symbol)
    spread = (tick.ask - tick.bid) / info.point
    spread_history.append(spread)
    if len(spread_history) > 50: spread_history.pop(0)
    avg_spread = sum(spread_history) / len(spread_history)
    
    if spread > max_spread_points or spread > avg_spread * spread_multiplier:
        print(f"Blocked: Spread volatile ({spread})")
        return True
    return False

def volatility_filter(df):
    last = df.iloc[-1]
    atr_mean = df['atr'].rolling(50).mean().iloc[-1]
    if last['atr'] > atr_mean * 1.8:
        print("Blocked: ATR spike")
        return True
    return False

# REFINED: Candle Break Logic
def check_signal(df):
    current = df.iloc[-1] # Currently forming
    prev = df.iloc[-2]    # Last closed
    
    # BUY: Trend Up + Pullback to EMA20 + Break Above Prev High
    if (prev['ema20'] > prev['ema50'] and 
        prev['low'] <= prev['ema20'] and prev['close'] >= prev['ema50'] and
        45 < prev['rsi'] < 60 and 
        current['close'] > prev['high']):
        return "buy"

    # SELL: Trend Down + Pullback to EMA20 + Break Below Prev Low
    if (prev['ema20'] < prev['ema50'] and 
        prev['high'] >= prev['ema20'] and prev['close'] <= prev['ema50'] and
        40 < prev['rsi'] < 55 and 
        current['close'] < prev['low']):
        return "sell"
    return None

def monitor_external_eas():
    positions = mt5.positions_get(symbol=symbol)
    if not positions: return
    for pos in positions:
        if pos.magic in EA_MAGIC_NUMBERS and pos.ticket not in logged_tickets:
            log_to_go("external", pos.profit, pos.magic)
            logged_tickets.add(pos.ticket)

def trailing_stop():
    positions = mt5.positions_get(symbol=symbol, magic=PYTHON_MAGIC)
    if not positions: return
    for pos in positions:
        tick = mt5.symbol_info_tick(symbol)
        if pos.type == 0: # BUY
            if (tick.bid - pos.price_open) > 0.5:
                new_sl = tick.bid - 0.3
                if pos.sl == 0 or new_sl > pos.sl: modify_sl(pos.ticket, new_sl)
        elif pos.type == 1: # SELL
            if (pos.price_open - tick.ask) > 0.5:
                new_sl = tick.ask + 0.3
                if pos.sl == 0 or new_sl < pos.sl: modify_sl(pos.ticket, new_sl)

def modify_sl(ticket, new_sl):
    request = {"action": mt5.TRADE_ACTION_SLTP, "position": ticket, "sl": new_sl, "tp": 0}
    mt5.order_send(request)

def place_trade(order_type, df):
    last = df.iloc[-1]
    tick = mt5.symbol_info_tick(symbol)
    price = tick.ask if order_type == "buy" else tick.bid
    sl = (price - (last['atr'] * atr_sl_multiplier)) if order_type == "buy" else (price + (last['atr'] * atr_sl_multiplier))
    tp = (price + (last['atr'] * atr_tp_multiplier)) if order_type == "buy" else (price - (last['atr'] * atr_tp_multiplier))

    request = {
        "action": mt5.TRADE_ACTION_DEAL,
        "symbol": symbol,
        "volume": fixed_lot,
        "type": mt5.ORDER_TYPE_BUY if order_type == "buy" else mt5.ORDER_TYPE_SELL,
        "price": price, "sl": sl, "tp": tp,
        "deviation": 20, "magic": PYTHON_MAGIC,
        "comment": "XAU $7 Safe",
        "type_time": mt5.ORDER_TIME_GTC, "type_filling": mt5.ORDER_FILLING_IOC,
    }
    result = mt5.order_send(request)
    if result.retcode == mt5.TRADE_RETCODE_DONE:
        trade_log.append(datetime.utcnow())
        log_to_go(order_type, 0.0, PYTHON_MAGIC)
        print(f"Successfully placed {order_type}")

# ==============================
# MAIN LOOP
# ==============================
while True:
    monitor_external_eas() # Check for Dark Venus/ExMachina trades
    
    if not session_filter() or news_filter():
        time.sleep(60); continue

    account = mt5.account_info()
    if ((daily_start_balance - account.balance) / daily_start_balance) * 100 >= max_daily_loss_percent:
        print("Daily loss limit reached."); break

    current_hr = datetime.utcnow().hour
    if sum(1 for t in trade_log if t.hour == current_hr) >= max_trades_per_hour:
        time.sleep(30); continue

    trailing_stop()
    df = get_data()

    if not volatility_filter(df) and not spread_filter():
        signal = check_signal(df)
        if signal and not order_book_filter(signal):
            place_trade(signal, df)

    time.sleep(5) # 5s polling for high-speed response
