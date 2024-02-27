# Trips and Signals WebApp

This is a [Next.js](https://nextjs.org/) project bootstrapped with
[`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

## Getting Started

The login view is built as Client Side code, based on example from wallet connect. This has been working with node v19.
Once logged in, the rest of the application is Server Side Rendered from Go with handlebars for templating. Client code is used as needed. 

### Build the login portion of the app

`npm install`
First, run the development server: 

```bash
npm run dev
# or
yarn dev
```

rRun the backend server, which also hosts the SSR webapp, navigate to directory where main.go is and run:

```bash
go run .
```

The backend Go server will be hosted on http://localhost:3003

## Using the Application

1. Sign in: Open the web application at http://localhost:3000 and click on the 'Connect Wallet' button
2. Sign the Message: After signing in, click on the 'Sign Message' button. A green verification banner should appear
3. The page will then redirect to api/vehicles/me to show a listing of the user's vehicles and their basic information
   
