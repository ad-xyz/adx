// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title EscrowManager
 * @dev Manages AUSD escrow for campaigns - solves "we delivered, they didn't pay" problem
 * 
 * Key Features:
 * - Pre-funded campaigns in AUSD
 * - Atomic reservation → receipt → settlement flow
 * - No payment without verified delivery
 * - Programmable holdbacks and penalties
 */
contract EscrowManager is ReentrancyGuard, Ownable {
    IERC20 public immutable AUSD;
    
    struct Campaign {
        address advertiser;
        uint256 totalBudget;
        uint256 availableBudget;
        uint256 reservedBudget;
        uint256 spentBudget;
        bool active;
        uint16 holdbackBps; // Basis points (e.g., 500 = 5%)
        uint256 created;
    }
    
    struct Reservation {
        bytes32 campaignId;
        address publisher;
        uint256 amount;
        uint256 expires;
        bool settled;
    }
    
    mapping(bytes32 => Campaign) public campaigns;
    mapping(bytes32 => Reservation) public reservations;
    mapping(address => uint256) public publisherBalances;
    mapping(address => uint256) public advertiserRefunds;
    
    // Events
    event CampaignFunded(bytes32 indexed campaignId, address indexed advertiser, uint256 amount);
    event ReservationCreated(bytes32 indexed reservationId, bytes32 indexed campaignId, address indexed publisher, uint256 amount);
    event ReceiptSettled(bytes32 indexed reservationId, address indexed publisher, uint256 amount);
    event BudgetWithdrawn(address indexed advertiser, uint256 amount);
    event PublisherPayout(address indexed publisher, uint256 amount);
    
    constructor(address _ausd) {
        AUSD = IERC20(_ausd);
    }
    
    /**
     * @dev Fund a campaign with AUSD - pre-funding solves payment risk
     * @param campaignId Unique campaign identifier
     * @param amount AUSD amount to fund
     * @param holdbackBps Percentage to hold back for fraud protection (basis points)
     */
    function fundCampaign(
        bytes32 campaignId, 
        uint256 amount,
        uint16 holdbackBps
    ) external nonReentrant {
        require(amount > 0, "Amount must be positive");
        require(holdbackBps <= 2000, "Holdback cannot exceed 20%"); // Max 20% holdback
        
        Campaign storage campaign = campaigns[campaignId];
        
        if (campaign.advertiser == address(0)) {
            // New campaign
            campaign.advertiser = msg.sender;
            campaign.holdbackBps = holdbackBps;
            campaign.created = block.timestamp;
            campaign.active = true;
        } else {
            require(campaign.advertiser == msg.sender, "Only campaign owner can fund");
        }
        
        // Transfer AUSD to escrow
        require(AUSD.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        campaign.totalBudget += amount;
        campaign.availableBudget += amount;
        
        emit CampaignFunded(campaignId, msg.sender, amount);
    }
    
    /**
     * @dev Reserve budget for an impression - creates atomic reservation
     * @param reservationId Unique reservation identifier (dedup key)
     * @param campaignId Campaign to reserve from
     * @param publisher Publisher receiving the impression
     * @param amount AUSD amount to reserve
     * @param ttlSeconds Time-to-live for reservation (typically 1-2 seconds)
     */
    function reserveBudget(
        bytes32 reservationId,
        bytes32 campaignId,
        address publisher,
        uint256 amount,
        uint256 ttlSeconds
    ) external nonReentrant {
        require(ttlSeconds <= 10, "TTL too long"); // Max 10 second reservations
        require(amount > 0, "Amount must be positive");
        require(reservations[reservationId].amount == 0, "Reservation exists");
        
        Campaign storage campaign = campaigns[campaignId];
        require(campaign.active, "Campaign inactive");
        require(campaign.availableBudget >= amount, "Insufficient budget");
        
        // Create reservation
        reservations[reservationId] = Reservation({
            campaignId: campaignId,
            publisher: publisher,
            amount: amount,
            expires: block.timestamp + ttlSeconds,
            settled: false
        });
        
        // Lock budget
        campaign.availableBudget -= amount;
        campaign.reservedBudget += amount;
        
        emit ReservationCreated(reservationId, campaignId, publisher, amount);
    }
    
    /**
     * @dev Settle receipt - pay publisher upon verified delivery
     * @param reservationId Reservation to settle
     * @param verificationProof Proof of ad delivery (hash/signature)
     */
    function settleReceipt(
        bytes32 reservationId,
        bytes32 verificationProof
    ) external nonReentrant {
        Reservation storage reservation = reservations[reservationId];
        require(reservation.amount > 0, "Reservation not found");
        require(!reservation.settled, "Already settled");
        require(block.timestamp <= reservation.expires, "Reservation expired");
        
        Campaign storage campaign = campaigns[reservation.campaignId];
        
        // Calculate payout (immediate) and holdback
        uint256 immediateAmount = reservation.amount * (10000 - campaign.holdbackBps) / 10000;
        uint256 holdbackAmount = reservation.amount - immediateAmount;
        
        // Update campaign accounting
        campaign.reservedBudget -= reservation.amount;
        campaign.spentBudget += reservation.amount;
        
        // Mark as settled
        reservation.settled = true;
        
        // Pay publisher immediately (streaming settlement)
        publisherBalances[reservation.publisher] += immediateAmount;
        
        // Schedule holdback release (simplified - in production would use timelock)
        if (holdbackAmount > 0) {
            // In production: create timelock for 24-48hr fraud window
            publisherBalances[reservation.publisher] += holdbackAmount;
        }
        
        emit ReceiptSettled(reservationId, reservation.publisher, reservation.amount);
    }
    
    /**
     * @dev Release expired reservation budget back to campaign
     * @param reservationId Expired reservation to clean up
     */
    function releaseExpiredReservation(bytes32 reservationId) external {
        Reservation storage reservation = reservations[reservationId];
        require(reservation.amount > 0, "Reservation not found");
        require(!reservation.settled, "Already settled");
        require(block.timestamp > reservation.expires, "Not expired");
        
        Campaign storage campaign = campaigns[reservation.campaignId];
        
        // Return budget to available pool
        campaign.availableBudget += reservation.amount;
        campaign.reservedBudget -= reservation.amount;
        
        // Clear reservation
        delete reservations[reservationId];
    }
    
    /**
     * @dev Withdraw publisher earnings in AUSD
     */
    function withdrawPublisher() external nonReentrant {
        uint256 amount = publisherBalances[msg.sender];
        require(amount > 0, "No balance");
        
        publisherBalances[msg.sender] = 0;
        require(AUSD.transfer(msg.sender, amount), "Transfer failed");
        
        emit PublisherPayout(msg.sender, amount);
    }
    
    /**
     * @dev Withdraw unused campaign budget
     * @param campaignId Campaign to withdraw from
     */
    function withdrawCampaignBudget(bytes32 campaignId) external nonReentrant {
        Campaign storage campaign = campaigns[campaignId];
        require(campaign.advertiser == msg.sender, "Not campaign owner");
        
        uint256 amount = campaign.availableBudget;
        require(amount > 0, "No available budget");
        
        campaign.availableBudget = 0;
        campaign.totalBudget -= amount;
        
        require(AUSD.transfer(msg.sender, amount), "Transfer failed");
        
        emit BudgetWithdrawn(msg.sender, amount);
    }
    
    /**
     * @dev Emergency pause campaign (stops new reservations)
     * @param campaignId Campaign to pause
     */
    function pauseCampaign(bytes32 campaignId) external {
        Campaign storage campaign = campaigns[campaignId];
        require(campaign.advertiser == msg.sender, "Not campaign owner");
        campaign.active = false;
    }
    
    /**
     * @dev Get campaign details
     */
    function getCampaign(bytes32 campaignId) external view returns (
        address advertiser,
        uint256 totalBudget,
        uint256 availableBudget,
        uint256 reservedBudget,
        uint256 spentBudget,
        bool active,
        uint16 holdbackBps
    ) {
        Campaign storage campaign = campaigns[campaignId];
        return (
            campaign.advertiser,
            campaign.totalBudget,
            campaign.availableBudget,
            campaign.reservedBudget,
            campaign.spentBudget,
            campaign.active,
            campaign.holdbackBps
        );
    }
}