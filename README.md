# Event Ticketing System Design

This document summarizes the high-level system design for a scalable event ticketing platform, detailing the functional and non-functional requirements, core data entities, API specifications, and the resulting architectural blueprint.

---

## 1. Requirements and Scope

### 1.1 Functional Requirements

The system must support the following core user actions:

* **Book tickets** for an event.
* **View an event** details (including venue, performer, and ticket information).
* **Search for events** based on various criteria.

### 1.2 Non-Functional Requirements (NFRs)

The architecture is designed to meet specific goals:

* **Consistency:** **Strong consistency** is required for **booking tickets** (write operations) to prevent overselling.
* **Availability:** **High availability** is required for **search and viewing events** (read operations).
* **Scalability:** The system must handle high traffic volume, particularly surges caused by popular event ticket sales, reflecting a high read-to-write ratio ($\text{read} \gg \text{write}$).

### 1.3 Out of Scope

The initial design iteration does not cover:

* GDPR compliance
* Advanced fault tolerance guarantees
* Detailed administrative and reporting features

---

## 2. Core Entities

The system's data model revolves around four primary entities, with **PostgreSQL** serving as the source of truth for transactional data.

| Entity | Role | Key Attributes (Simplified) |
| :--- | :--- | :--- |
| **Event** | The primary item being sold (e.g., a concert). | `id`, `venueId`, `performerId`, `name`, `description` |
| **Venue** | The physical location where the event takes place. | `id`, `location`, `seatMap` |
| **Performer** | The artist or group performing at the event. | `id`, `name`, `description` |
| **Ticket** | The purchasable item. | `id`, `eventId`, `seat`, `price`, `status` (available/booked), `userId` |

---

## 3. API Design

The public API is divided into read/search and write/booking operations. Authentication is typically handled via JWT or a session token passed in the request header.

### Search & Viewing Endpoints

| HTTP Method | Endpoint | Query Parameters | Response/Output |
| :--- | :--- | :--- | :--- |
| **GET** | `/event/{eventId}` | None | Full Event + Venue + Performer + Ticket List |
| **GET** | `/search` | `term`, `location`, `type`, `date` | List of Partial Event objects |

### Booking Endpoints

| HTTP Method | Endpoint | Authentication | Body/Details |
| :--- | :--- | :--- | :--- |
| **POST** | `/booking/reserve` | `JWT` / `sessionToken` | `{ ticketId: ... }` (Reserves the ticket) |
| **PUT** | `/booking/confirm` | `JWT` / `sessionToken` | `{ ticketId: ..., paymentDetails: (stripe) }` (Confirms and finalizes payment) |

---

## 4. System Architecture Diagram (Mermaid Flowchart)

The architecture uses a microservices approach to separate concerns and scale read/write operations independently.

```mermaid
graph TD
    % Define the components
    A[Client]
    B[CDN]
    C[API Gateway: Auth, Rate Limiting, Routing]
    K[Virtual Waiting Queue]

    subgraph Data Persistence
        G(Database: Postgres)
        H(Elasticsearch / OpenSearch)
    end

    subgraph Core Services
        D[Search Service]
        E[Event CRUD Service]
        F[Booking Service]
    end

    subgraph Caching & External
        I(Tickets Lock: Redis TTL 10M)
        J[Stripe]
        L[CDC]
    end

    % Define Flows
    A --> B
    B --> C

    % Read/Search Path
    C -- GET /search --> D
    D --> H
    C -- GET /event/{id} --> E
    E -- Read --> G

    % Write/Booking Path
    C -- POST/PUT /booking --> K
    K --> F
    F -- Reserve Lock --> I
    F -- Final Write --> G
    F -- Payment --> J

    % Asynchronous Data Flow
    G -- Data Changes --> L
    L --> H
    
    classDef service fill:#e0f7fa,stroke:#00bcd4,stroke-width:2px;
    class D,E,F service
    classDef db fill:#fff3e0,stroke:#ff9800,stroke-width:2px;
    class G,H,I db
```

### Component Summary

  * **API Gateway:** Serves as the single entry point, handling security (authentication) and routing requests to the appropriate backend service.
  * **Search Service:** Queries the high-performance **Elasticsearch** cluster for event discovery, ensuring quick response times for reads.
  * **Event CRUD Service:** Directly interacts with the relational **PostgreSQL** database, ensuring data integrity for event details.
  * **Booking Service:** The critical write path. It uses the **Virtual Waiting Queue** to manage load and **Redis** for temporary ticket locks before committing the transaction to Postgres and processing payment via **Stripe**.
  * **CDC (Change Data Capture):** An asynchronous process that streams committed changes from the Postgres database to keep the Elasticsearch search index up-to-date (eventually consistent).

-----

## 5\. Technology Summary

| Component | Technology | Rationale |
| :--- | :--- | :--- |
| **Primary Database** | PostgreSQL | Strong consistency, data integrity, and ACID compliance for booking transactions. |
| **Search Engine** | Elasticsearch/OpenSearch | Scalable, highly available, and flexible full-text search capabilities. |
| **Caching/Locking** | Redis | Fast, in-memory storage for distributed locking (ticket reservation) and potential high-volume caching. |

-----