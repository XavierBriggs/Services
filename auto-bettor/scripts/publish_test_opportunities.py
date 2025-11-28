#!/usr/bin/env python3
"""
Test script for publishing opportunities to Redis for auto-bettor testing
"""

import json
import redis
import sys
from datetime import datetime, timedelta

def create_edge_opportunity(opp_id, edge_pct=3.5, book="betus"):
    """Create a test edge opportunity"""
    point = 5.5
    return {
        "id": opp_id,
        "opportunity_type": "edge",
        "sport_key": "basketball_nba",
        "event_id": f"test_event_{opp_id}",
        "market_key": "spreads",
        "edge_pct": edge_pct,
        "fair_price": 0,
        "detected_at": datetime.now().isoformat() + "Z",
        "data_age_seconds": 5,
        "game_start_time": (datetime.now() + timedelta(hours=2)).isoformat() + "Z",
        "legs": [{
            "id": opp_id * 10,
            "book_key": book,
            "outcome_name": f"Lakers +{point}",
            "price": 110,
            "point": point,
            "leg_edge_pct": edge_pct
        }]
    }

def create_middle_opportunity(opp_id):
    """Create a test middle opportunity"""
    return {
        "id": opp_id,
        "opportunity_type": "middle",
        "sport_key": "basketball_nba",
        "event_id": f"test_event_{opp_id}",
        "market_key": "spreads",
        "edge_pct": 5.0,
        "fair_price": 0,
        "detected_at": datetime.now().isoformat() + "Z",
        "data_age_seconds": 5,
        "game_start_time": (datetime.now() + timedelta(hours=3)).isoformat() + "Z",
        "legs": [
            {
                "id": opp_id * 10 + 1,
                "book_key": "betus",
                "outcome_name": "Celtics -5.5",
                "price": -110,
                "point": -5.5,
                "leg_edge_pct": 2.5
            },
            {
                "id": opp_id * 10 + 2,
                "book_key": "bovada",
                "outcome_name": "Celtics +6.5",
                "price": -110,
                "point": 6.5,
                "leg_edge_pct": 2.5
            }
        ]
    }

def create_scalp_opportunity(opp_id):
    """Create a test scalp opportunity"""
    return {
        "id": opp_id,
        "opportunity_type": "scalp",
        "sport_key": "basketball_nba",
        "event_id": f"test_event_{opp_id}",
        "market_key": "h2h",
        "edge_pct": 2.0,
        "fair_price": 0,
        "detected_at": datetime.now().isoformat() + "Z",
        "data_age_seconds": 5,
        "game_start_time": (datetime.now() + timedelta(hours=4)).isoformat() + "Z",
        "legs": [
            {
                "id": opp_id * 10 + 1,
                "book_key": "betus",
                "outcome_name": "Warriors ML",
                "price": 110,
                "point": None,
                "leg_edge_pct": 0
            },
            {
                "id": opp_id * 10 + 2,
                "book_key": "bovada",
                "outcome_name": "Clippers ML",
                "price": 110,
                "point": None,
                "leg_edge_pct": 0
            }
        ]
    }

def main():
    # Connect to Redis
    try:
        r = redis.Redis(host='localhost', port=6379, decode_responses=True)
        r.ping()
        print("✓ Connected to Redis")
    except Exception as e:
        print(f"❌ Failed to connect to Redis: {e}")
        sys.exit(1)

    print("\nPublishing test opportunities...\n")

    # Publish edge opportunity
    edge_opp = create_edge_opportunity(1001, edge_pct=3.5)
    r.xadd('opportunities.detected', {'opportunity': json.dumps(edge_opp)})
    print("✓ Published EDGE opportunity (ID: 1001, Edge: 3.5%, Book: betus)")

    # Publish middle opportunity
    middle_opp = create_middle_opportunity(1002)
    r.xadd('opportunities.detected', {'opportunity': json.dumps(middle_opp)})
    print("✓ Published MIDDLE opportunity (ID: 1002, Edge: 5.0%, Books: betus + bovada)")

    # Publish scalp opportunity
    scalp_opp = create_scalp_opportunity(1003)
    r.xadd('opportunities.detected', {'opportunity': json.dumps(scalp_opp)})
    print("✓ Published SCALP opportunity (ID: 1003, Profit: 2.0%, Books: betus + bovada)")

    # Publish low edge opportunity (should be filtered)
    low_edge = create_edge_opportunity(1004, edge_pct=1.0)
    r.xadd('opportunities.detected', {'opportunity': json.dumps(low_edge)})
    print("✓ Published LOW EDGE opportunity (ID: 1004, Edge: 1.0%) - should be filtered")

    # Publish stale data opportunity (should be filtered)
    stale_opp = create_edge_opportunity(1005, edge_pct=3.5)
    stale_opp["data_age_seconds"] = 60  # Too old
    r.xadd('opportunities.detected', {'opportunity': json.dumps(stale_opp)})
    print("✓ Published STALE opportunity (ID: 1005, Age: 60s) - should be filtered")

    print("\n✅ Published 5 test opportunities")
    print("\nCheck auto-bettor logs to see processing results.")
    print("Run 'python3 check_results.py' to verify database records.")

if __name__ == "__main__":
    main()


