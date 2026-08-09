# Phase 1 PRD --- Enterprise Inventory Management System (Modular Monolith)

**Version:** 1.0\
**Phase:** 1 --- Foundation\
**Duration:** 4--8 Weeks\
**Difficulty:** Beginner → Intermediate

------------------------------------------------------------------------

# 1. Vision

Build a production-quality Inventory Management System that teaches
idiomatic Go while following enterprise software engineering practices.
This phase intentionally uses a modular monolith before introducing
microservices in later phases.

------------------------------------------------------------------------

# 2. Objectives

## Primary Goals

-   Learn Go fundamentals through a real project.
-   Build a maintainable REST API.
-   Apply Clean Architecture.
-   Practice Repository and Service patterns.
-   Implement secure authentication.
-   Create a modern React dashboard.
-   Produce an interview-ready portfolio project.

------------------------------------------------------------------------

# 3. Success Criteria

-   CRUD features completed.
-   Authentication with JWT.
-   PostgreSQL database.
-   Dockerized application.
-   Swagger documentation.
-   Unit test coverage ≥ 80%.
-   Clean Architecture with dependency injection.
-   Responsive React frontend.

------------------------------------------------------------------------

# 4. Technology Stack

## Backend

-   Go 1.24+
-   Gin
-   GORM
-   PostgreSQL 17
-   pgx
-   Viper
-   Zap
-   validator/v10
-   golang-jwt/jwt/v5
-   bcrypt
-   google/uuid
-   Swaggo
-   Testify
-   Air
-   Delve
-   golangci-lint

## Frontend

-   React 19
-   TypeScript
-   Vite
-   Tailwind CSS v4
-   shadcn/ui
-   TanStack Query
-   React Router
-   React Hook Form
-   Zod
-   Lucide React

## Development

-   Docker
-   Docker Compose
-   Git
-   GitHub Actions
-   Makefile

------------------------------------------------------------------------

# 5. Functional Requirements

## Authentication

-   Register
-   Login
-   Logout
-   Refresh Token
-   Change Password
-   Update Profile

## Product

-   Create Product
-   Update Product
-   Delete Product
-   Product Details
-   Search
-   Pagination
-   Sorting
-   Filtering

## Category

-   CRUD Categories

## Inventory

-   Stock In
-   Stock Out
-   Inventory History
-   Low Stock Indicator

## Dashboard

-   Total Products
-   Total Categories
-   Inventory Value
-   Low Stock Summary
-   Recent Activities

------------------------------------------------------------------------

# 6. Non-functional Requirements

-   RESTful API
-   Response time \<300ms (local)
-   Structured logging
-   Input validation
-   Secure password hashing
-   Environment configuration
-   Docker support
-   Mobile responsive UI

------------------------------------------------------------------------

# 7. Database Tables

-   users
-   roles
-   products
-   categories
-   inventory
-   inventory_transactions
-   refresh_tokens
-   activity_logs

------------------------------------------------------------------------

# 8. Backend Architecture

``` text
HTTP Request
    ↓
Gin Router
    ↓
Middleware
    ↓
Handler
    ↓
Service
    ↓
Repository Interface
    ↓
Repository (GORM)
    ↓
PostgreSQL
```

------------------------------------------------------------------------

# 9. Folder Structure

``` text
cmd/server

internal/
  auth/
  user/
  product/
  category/
  inventory/
  shared/
    config/
    middleware/
    logger/
    database/
    response/
    validator/

docs/
migrations/
web/
tests/
```

------------------------------------------------------------------------

# 10. Go Concepts

-   Packages
-   Modules
-   Structs
-   Interfaces
-   Methods
-   Pointer Receivers
-   Value Receivers
-   Context
-   Error Wrapping
-   Dependency Injection
-   Generics
-   Middleware
-   Transactions
-   Unit Testing

------------------------------------------------------------------------

# 11. Security

-   JWT Authentication
-   Refresh Tokens
-   RBAC
-   Password Hashing
-   Input Validation
-   SQL Injection Prevention
-   Secure Headers
-   CORS

------------------------------------------------------------------------

# 12. API Modules

-   Authentication API
-   User API
-   Product API
-   Category API
-   Inventory API
-   Dashboard API

------------------------------------------------------------------------

# 13. Frontend Pages

-   Login
-   Register
-   Dashboard
-   Products
-   Categories
-   Inventory
-   Profile
-   Settings

Reusable Components

-   Sidebar
-   Navbar
-   Data Table
-   Search Bar
-   Pagination
-   Modal
-   Toast
-   Loading Spinner
-   Empty State

------------------------------------------------------------------------

# 14. Testing

-   Repository Tests
-   Service Tests
-   Handler Tests
-   Middleware Tests

Coverage Target: **80%+**

------------------------------------------------------------------------

# 15. Documentation

-   README
-   ER Diagram
-   API Documentation (Swagger)
-   Architecture Diagram
-   Installation Guide

------------------------------------------------------------------------

# 16. Milestones

1.  Project Initialization
2.  Database Configuration
3.  Authentication Module
4.  User Module
5.  Product Module
6.  Category Module
7.  Inventory Module
8.  Dashboard
9.  Testing
10. Swagger
11. Docker
12. CI Preparation

------------------------------------------------------------------------

# 17. Definition of Done

-   All CRUD operations work.
-   Authentication complete.
-   Validation implemented.
-   Clean Architecture maintained.
-   Docker builds successfully.
-   Swagger generated.
-   Tests pass.
-   Code formatted with gofmt.
-   golangci-lint passes without critical issues.
-   README documents setup and architecture.

------------------------------------------------------------------------

# Phase 1 Outcome

After completing Phase 1 you should confidently understand:

-   Idiomatic Go fundamentals
-   REST API development with Gin
-   GORM and PostgreSQL
-   Clean Architecture
-   Dependency Injection
-   Repository & Service Pattern
-   JWT Authentication
-   Docker fundamentals
-   Debugging with Delve
-   React integration with a Go backend

Estimated competency after completion: **\~90/100** for junior Go
backend development.
