// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC1155/ERC1155.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title AdSlotSFT
 * @dev Semi-Fungible Tokens for ad inventory - "Hyperliquid for ADX" primitives
 * 
 * Each token represents: "up to N qualified impressions for predicate P 
 * within window [t0,t1] on placement X with quality constraints"
 */
contract AdSlotSFT is ERC1155, Ownable, ReentrancyGuard {
    
    struct AdSlot {
        address publisher;
        string placement;          // e.g. "ctv-preroll", "banner-300x250"
        bytes32 targetingHash;     // Hash of targeting predicate (geo, demo, etc.)
        uint256 startTime;         // Window start timestamp
        uint256 endTime;           // Window end timestamp  
        uint256 maxImpressions;    // Total supply
        uint256 minViewability;    // Quality floor (e.g., 70 = 70%)
        uint256 floorCPM;          // Minimum price (in AUSD wei)
        bool active;
        uint256 delivered;         // Impressions delivered so far
    }
    
    struct TargetingPredicate {
        string[] geoTargets;       // ["US", "CA", "UK"]
        string[] deviceTypes;      // ["CTV", "mobile", "desktop"] 
        string[] categories;       // ["IAB1", "IAB2"]
        uint256 minAge;
        uint256 maxAge;
        bytes32 customHash;        // For complex predicates
    }
    
    mapping(uint256 => AdSlot) public adSlots;
    mapping(uint256 => TargetingPredicate) public targeting;
    mapping(address => bool) public authorizedMinters;
    
    uint256 public nextSlotId = 1;
    
    // Events
    event AdSlotCreated(
        uint256 indexed slotId, 
        address indexed publisher, 
        string placement,
        uint256 startTime,
        uint256 endTime,
        uint256 maxImpressions
    );
    
    event ImpressionDelivered(uint256 indexed slotId, uint256 count);
    event SlotTransferred(uint256 indexed slotId, address from, address to, uint256 amount);
    
    constructor() ERC1155("https://api.adx.com/metadata/{id}") {}
    
    /**
     * @dev Create new ad slot token (perishable inventory)
     * @param publisher Publisher owning the inventory
     * @param placement Placement identifier
     * @param targetingPredicate Targeting constraints
     * @param startTime Delivery window start
     * @param endTime Delivery window end (perishable!)
     * @param maxImpressions Total available impressions
     * @param minViewability Minimum viewability percentage
     * @param floorCPM Floor price in AUSD
     */
    function createAdSlot(
        address publisher,
        string memory placement,
        TargetingPredicate memory targetingPredicate,
        uint256 startTime,
        uint256 endTime,
        uint256 maxImpressions,
        uint256 minViewability,
        uint256 floorCPM
    ) external returns (uint256 slotId) {
        require(authorizedMinters[msg.sender] || msg.sender == owner(), "Not authorized");
        require(startTime < endTime, "Invalid time window");
        require(endTime > block.timestamp, "Window already ended");
        require(maxImpressions > 0, "No impressions");
        require(minViewability <= 100, "Invalid viewability");
        
        slotId = nextSlotId++;
        
        // Hash the targeting predicate for efficient matching
        bytes32 targetingHash = keccak256(abi.encode(
            targetingPredicate.geoTargets,
            targetingPredicate.deviceTypes, 
            targetingPredicate.categories,
            targetingPredicate.minAge,
            targetingPredicate.maxAge,
            targetingPredicate.customHash
        ));
        
        adSlots[slotId] = AdSlot({
            publisher: publisher,
            placement: placement,
            targetingHash: targetingHash,
            startTime: startTime,
            endTime: endTime,
            maxImpressions: maxImpressions,
            minViewability: minViewability,
            floorCPM: floorCPM,
            active: true,
            delivered: 0
        });
        
        targeting[slotId] = targetingPredicate;
        
        // Mint SFT to publisher
        _mint(publisher, slotId, maxImpressions, "");
        
        emit AdSlotCreated(slotId, publisher, placement, startTime, endTime, maxImpressions);
    }
    
    /**
     * @dev Record impression delivery (reduces available supply)
     * @param slotId Ad slot being delivered
     * @param count Number of impressions delivered
     */
    function recordDelivery(uint256 slotId, uint256 count) external {
        require(authorizedMinters[msg.sender], "Not authorized");
        
        AdSlot storage slot = adSlots[slotId];
        require(slot.active, "Slot inactive");
        require(block.timestamp >= slot.startTime, "Window not started");
        require(block.timestamp <= slot.endTime, "Window expired");
        require(slot.delivered + count <= slot.maxImpressions, "Exceeds capacity");
        
        slot.delivered += count;
        
        // Burn delivered tokens from circulation
        _burn(slot.publisher, slotId, count);
        
        emit ImpressionDelivered(slotId, count);
    }
    
    /**
     * @dev Check if impression matches slot targeting
     * @param slotId Ad slot to check
     * @param geo User geography
     * @param deviceType Device type
     * @param age User age
     * @param categories Content categories
     * @return matches True if impression qualifies
     */
    function matchesTargeting(
        uint256 slotId,
        string memory geo,
        string memory deviceType, 
        uint256 age,
        string[] memory categories
    ) external view returns (bool matches) {
        TargetingPredicate storage pred = targeting[slotId];
        
        // Check geography
        bool geoMatch = pred.geoTargets.length == 0;
        for (uint i = 0; i < pred.geoTargets.length && !geoMatch; i++) {
            if (keccak256(bytes(pred.geoTargets[i])) == keccak256(bytes(geo))) {
                geoMatch = true;
            }
        }
        if (!geoMatch) return false;
        
        // Check device type
        bool deviceMatch = pred.deviceTypes.length == 0;
        for (uint i = 0; i < pred.deviceTypes.length && !deviceMatch; i++) {
            if (keccak256(bytes(pred.deviceTypes[i])) == keccak256(bytes(deviceType))) {
                deviceMatch = true;
            }
        }
        if (!deviceMatch) return false;
        
        // Check age
        if (pred.minAge > 0 && age < pred.minAge) return false;
        if (pred.maxAge > 0 && age > pred.maxAge) return false;
        
        // Check categories (any match)
        if (pred.categories.length > 0) {
            bool categoryMatch = false;
            for (uint i = 0; i < categories.length && !categoryMatch; i++) {
                for (uint j = 0; j < pred.categories.length; j++) {
                    if (keccak256(bytes(pred.categories[j])) == keccak256(bytes(categories[i]))) {
                        categoryMatch = true;
                        break;
                    }
                }
            }
            if (!categoryMatch) return false;
        }
        
        return true;
    }
    
    /**
     * @dev Calculate time-decay price (perishable inventory gets cheaper)
     * @param slotId Ad slot to price
     * @return currentPrice Price adjusted for time decay
     */
    function getCurrentPrice(uint256 slotId) external view returns (uint256 currentPrice) {
        AdSlot storage slot = adSlots[slotId];
        
        if (!slot.active || block.timestamp > slot.endTime) {
            return 0; // Expired = worthless
        }
        
        if (block.timestamp < slot.startTime) {
            return slot.floorCPM; // Not started = full price
        }
        
        // Linear time decay: price drops as expiration approaches
        uint256 timeRemaining = slot.endTime - block.timestamp;
        uint256 totalWindow = slot.endTime - slot.startTime;
        
        // Price = floor + (market_premium * time_remaining / total_window)
        // For simplicity, assume 50% premium over floor that decays
        uint256 premium = slot.floorCPM / 2;
        currentPrice = slot.floorCPM + (premium * timeRemaining / totalWindow);
    }
    
    /**
     * @dev Authorize minter (exchange contracts, oracles)
     */
    function setAuthorizedMinter(address minter, bool authorized) external onlyOwner {
        authorizedMinters[minter] = authorized;
    }
    
    /**
     * @dev Emergency deactivate slot
     */
    function deactivateSlot(uint256 slotId) external {
        AdSlot storage slot = adSlots[slotId];
        require(msg.sender == slot.publisher || msg.sender == owner(), "Not authorized");
        slot.active = false;
    }
    
    /**
     * @dev Get remaining supply for slot
     */
    function remainingSupply(uint256 slotId) external view returns (uint256) {
        AdSlot storage slot = adSlots[slotId];
        if (slot.maxImpressions > slot.delivered) {
            return slot.maxImpressions - slot.delivered;
        }
        return 0;
    }
    
    /**
     * @dev Check if slot is expired
     */
    function isExpired(uint256 slotId) external view returns (bool) {
        return block.timestamp > adSlots[slotId].endTime;
    }
    
    /**
     * @dev Override to add transfer restrictions (optional)
     */
    function safeTransferFrom(
        address from,
        address to,
        uint256 id,
        uint256 amount,
        bytes memory data
    ) public override {
        require(adSlots[id].active, "Slot inactive");
        require(block.timestamp <= adSlots[id].endTime, "Slot expired");
        
        super.safeTransferFrom(from, to, id, amount, data);
        
        emit SlotTransferred(id, from, to, amount);
    }
}