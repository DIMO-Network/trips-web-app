# Trips Sandbox WebApp

Welcome to the Trips Sandbox repository! This project is a React + TypeScript + Vite application designed to interface with the DIMO Network's Trips API and Telemetry API. The application pulls trip data and displays the trips using Mapbox for each specific vehicle ID.

## Table of Contents

- [Introduction](#introduction)
- [Getting Started](#getting-started)
- [Running Locally](#running-locally)
- [Deployment](#deployment)
- [Contributing](#contributing)

## Introduction

The Trips Sandbox allows users to view and analyze trip data for specific vehicles. It uses Mapbox for visualizing the trips on a map. This README provides instructions on how to set up the project locally, details on deployment, and guidelines for contributing.

## Getting Started

### Prerequisites

Before you begin, ensure you have the following installed on your system:

- [Node.js](https://nodejs.org/en/download/)
- [npm](https://www.npmjs.com/get-npm) or [Yarn](https://yarnpkg.com/getting-started/install)
- [Go](https://golang.org/dl/)

### Installation

1. Clone the repository:
    ```sh
    git clone https://github.com/DIMO-Network/trips-web-app.git
    cd trips-web-app
    ```

2. Install the dependencies for the frontend:
    ```sh
    cd app-login
    npm install
    # or
    yarn install
    ```

## Running Locally

1. Create a `.env` file in the `app-login` directory

2. Start the frontend development server:
    ```sh
    npm run dev
    # or
    yarn dev
    ```

3. Navigate to the directory where `main.go` is located and run the backend server:
    ```sh
    go run ./cmd/trips-web-app
    ```

   The backend Go server will be hosted on [http://localhost:3003](http://localhost:3003).

## Deployment

Deploying the Trips Sandbox involves a few steps:

1. **Build the frontend**:
    ```sh
    cd app-login
    npm run build
    # or
    yarn build
    ```

   This will create a `dist` directory with the production build of your app.

2. **Copy the build output**:
    ```sh
    cp -r dist ../api
    ```

3. **Run the backend server**:
    ```sh
    cd ../api
    go run ./cmd/trips-web-app
    ```

### Deployment Challenges

- **Environment Variables**: Ensure all necessary environment variables are correctly set up in your hosting environment.
- **API Access**: Make sure the hosting service can access the DIMO Network's Trips API and Telemetry API.
- **SSL/TLS**: Secure your application using SSL/TLS if it's accessible over the internet.


## Contributing

We welcome contributions to the Trips Sandbox! Please follow these steps to contribute:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature-name`).
3. Commit your changes (`git commit -am 'Add some feature'`).
4. Push to the branch (`git push origin feature/your-feature-name`).
5. Create a new Pull Request.

