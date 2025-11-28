#!/usr/bin/env python3
"""
Check auto-bettor results in the database
"""

import psycopg2
import os
from tabulate import tabulate

def connect_db():
    """Connect to Holocron database"""
    dsn = os.getenv("HOLOCRON_DSN", "postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable")
    return psycopg2.connect(dsn)

def check_decisions(conn):
    """Check recent auto-betting decisions"""
    cursor = conn.cursor()
    cursor.execute("""
        SELECT 
            id,
            opportunity_id,
            decision,
            decision_reason,
            calculated_stake,
            calculated_edge,
            execution_time_ms,
            created_at
        FROM auto_betting_decisions
        WHERE created_at > NOW() - INTERVAL '5 minutes'
        ORDER BY created_at DESC
    """)
    
    rows = cursor.fetchall()
    if rows:
        headers = ["ID", "Opp ID", "Decision", "Reason", "Stake", "Edge%", "Exec (ms)", "Created"]
        print("\n=== Recent Decisions ===")
        print(tabulate(rows, headers=headers, tablefmt="grid"))
    else:
        print("\n❌ No recent decisions found")

def check_state(conn):
    """Check current auto-betting state"""
    cursor = conn.cursor()
    cursor.execute("""
        SELECT 
            total_exposure,
            bets_placed_last_hour,
            bets_placed_today,
            todays_pnl,
            current_loss_streak,
            is_paused,
            pause_reason
        FROM auto_betting_state
        WHERE user_id = 'default'
    """)
    
    row = cursor.fetchone()
    if row:
        print("\n=== Current State ===")
        print(f"Total Exposure:      ${row[0]:.2f}")
        print(f"Bets (Last Hour):    {row[1]}")
        print(f"Bets (Today):        {row[2]}")
        print(f"Today's P&L:         ${row[3]:.2f}")
        print(f"Loss Streak:         {row[4]}")
        print(f"Paused:              {row[5]}")
        if row[6]:
            print(f"Pause Reason:        {row[6]}")
    else:
        print("\n❌ No state found")

def check_bets(conn):
    """Check recently placed bets"""
    cursor = conn.cursor()
    cursor.execute("""
        SELECT 
            id,
            opportunity_id,
            book_key,
            stake,
            status,
            created_at
        FROM bets
        WHERE created_at > NOW() - INTERVAL '5 minutes'
        ORDER BY created_at DESC
    """)
    
    rows = cursor.fetchall()
    if rows:
        headers = ["Bet ID", "Opp ID", "Book", "Stake", "Status", "Created"]
        print("\n=== Recently Placed Bets ===")
        print(tabulate(rows, headers=headers, tablefmt="grid"))
    else:
        print("\n⚠️  No recent bets placed")

def main():
    try:
        print("Connecting to Holocron database...")
        conn = connect_db()
        print("✓ Connected\n")
        
        check_decisions(conn)
        check_state(conn)
        check_bets(conn)
        
        print("\n✅ Check complete")
        
        conn.close()
    except Exception as e:
        print(f"❌ Error: {e}")

if __name__ == "__main__":
    main()


