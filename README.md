# Trips and Signals WebApp

This is a React + TypeScript + Vite application.

### Build the login portion of the app
Inside of app-login folder, create `.env` file then

`npm install`
First, run the development server: 

```bash
npm run dev
# or
yarn dev
```

Run the backend server, which also hosts the SSR webapp, navigate to directory where main.go is and run:

```bash
go run ./cmd/trips-web-app
```

The backend Go server will be hosted on http://localhost:3003

## Using the Application

1. inside app-login folter run `npm run build` and copy dist folder to api folder
2. run backend.
   
