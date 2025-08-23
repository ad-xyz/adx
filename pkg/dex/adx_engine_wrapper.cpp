#include "adx_matching_engine.h"
#include <cstring>
#include <memory>
#include <sstream>

extern "C" {

// C wrapper structures
typedef struct {
    char* id;
    char* ad_slot_id;
    double price;
    uint64_t quantity;
    uint64_t timestamp_ms;
    uint8_t is_buy;
    char* advertiser_id;
    char* targeting_json;
} COrder;

typedef struct {
    char* slot_id;
    double clearing_price;
    uint64_t total_volume;
    uint32_t matched_orders;
    char* winners_json;
} CBatchResult;

// Create a new ADX matching engine
void* create_adx_engine() {
    try {
        return new ADXMatchingEngine();
    } catch (...) {
        return nullptr;
    }
}

// Destroy the ADX matching engine
void destroy_adx_engine(void* engine) {
    if (engine) {
        delete static_cast<ADXMatchingEngine*>(engine);
    }
}

// Add an order to the engine
int add_order(void* engine, COrder* c_order) {
    if (!engine || !c_order) return -1;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        
        // Parse targeting JSON
        nlohmann::json targeting;
        if (c_order->targeting_json) {
            targeting = nlohmann::json::parse(c_order->targeting_json);
        }
        
        // Create order
        Order order;
        order.id = c_order->id ? c_order->id : "";
        order.ad_slot_id = c_order->ad_slot_id ? c_order->ad_slot_id : "";
        order.price = c_order->price;
        order.quantity = c_order->quantity;
        order.timestamp_ms = c_order->timestamp_ms;
        order.is_buy = c_order->is_buy != 0;
        order.advertiser_id = c_order->advertiser_id ? c_order->advertiser_id : "";
        order.targeting = targeting;
        
        return adx_engine->addOrder(order) ? 0 : -2;
    } catch (...) {
        return -3;
    }
}

// Run a batch auction
CBatchResult* run_batch_auction(void* engine, const char* slot_id, uint32_t batch_ms) {
    if (!engine || !slot_id) return nullptr;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        
        // Run auction
        auto result = adx_engine->runBatchAuction(slot_id, batch_ms);
        
        // Convert to C structure
        auto* c_result = new CBatchResult();
        c_result->slot_id = strdup(result.slot_id.c_str());
        c_result->clearing_price = result.clearing_price;
        c_result->total_volume = result.total_volume;
        c_result->matched_orders = result.matched_orders;
        
        // Convert winners to JSON
        nlohmann::json winners_json = nlohmann::json::array();
        for (const auto& match : result.winners) {
            winners_json.push_back({
                {"order_id", match.order_id},
                {"advertiser_id", match.advertiser_id},
                {"price", match.price},
                {"quantity", match.quantity}
            });
        }
        
        std::string winners_str = winners_json.dump();
        c_result->winners_json = strdup(winners_str.c_str());
        
        return c_result;
    } catch (...) {
        return nullptr;
    }
}

// Free batch result memory
void free_batch_result(CBatchResult* result) {
    if (result) {
        if (result->slot_id) free(result->slot_id);
        if (result->winners_json) free(result->winners_json);
        delete result;
    }
}

// Calculate time decay
double calculate_time_decay(void* engine, const char* slot_id, uint64_t timestamp_ms) {
    if (!engine || !slot_id) return 0.0;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        return adx_engine->calculateTimeDecay(slot_id, timestamp_ms);
    } catch (...) {
        return 0.0;
    }
}

// Cancel an order
int cancel_order(void* engine, const char* order_id) {
    if (!engine || !order_id) return -1;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        return adx_engine->cancelOrder(order_id) ? 0 : -2;
    } catch (...) {
        return -3;
    }
}

// Get order book as JSON
char* get_order_book_json(void* engine, const char* slot_id) {
    if (!engine || !slot_id) return nullptr;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        auto order_book = adx_engine->getOrderBook(slot_id);
        
        nlohmann::json json_book = {
            {"slot_id", order_book.slot_id},
            {"bids", nlohmann::json::array()},
            {"asks", nlohmann::json::array()},
            {"last_update", order_book.last_update_ms}
        };
        
        for (const auto& level : order_book.bids) {
            json_book["bids"].push_back({
                {"price", level.price},
                {"quantity", level.quantity},
                {"orders", level.order_count}
            });
        }
        
        for (const auto& level : order_book.asks) {
            json_book["asks"].push_back({
                {"price", level.price},
                {"quantity", level.quantity},
                {"orders", level.order_count}
            });
        }
        
        std::string json_str = json_book.dump();
        return strdup(json_str.c_str());
    } catch (...) {
        return nullptr;
    }
}

// Get engine metrics as JSON
char* get_metrics_json(void* engine) {
    if (!engine) return nullptr;
    
    try {
        auto* adx_engine = static_cast<ADXMatchingEngine*>(engine);
        auto metrics = adx_engine->getMetrics();
        
        nlohmann::json json_metrics = {
            {"total_orders", metrics.total_orders},
            {"active_orders", metrics.active_orders},
            {"total_auctions", metrics.total_auctions},
            {"total_volume", metrics.total_volume},
            {"avg_latency_us", metrics.avg_latency_us},
            {"p99_latency_us", metrics.p99_latency_us},
            {"gpu_utilization", metrics.gpu_utilization},
            {"orders_per_second", metrics.orders_per_second},
            {"auctions_per_second", metrics.auctions_per_second}
        };
        
        std::string json_str = json_metrics.dump();
        return strdup(json_str.c_str());
    } catch (...) {
        return nullptr;
    }
}

} // extern "C"