# Trips and Signals WebApp

This is a [Next.js](https://nextjs.org/) project bootstrapped with
[`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

## Getting Started

Make sure to have react and next installed. 
First, run the development server: 

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
```

To run the backend server, navigate to directory where main.go is and run:

```bash
go run .
```

The backend Fiber server will be hosted on http://localhost:3003

## Using the Application

1. Sign in: Open the web application at http://localhost:3000 and click on the 'Connect Wallet' button
2. Sign the Message: After signing in, click on the 'Sign Message' button. A green verification banner should appear, displaying the signature
3. View the token: To view the token, open the web inspector in the browser. In the network tab, find the response of the submit_challenge endpoint. The token will be included in this response
   
