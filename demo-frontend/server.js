const express = require('express');
const path = require('path');
const app = express();
const port = 3001;

app.use(express.static('public'));

// Routes
app.get('/franchiser/login', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'login.html'));
});

app.get('/franchiser/oauth', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'oauth-callback.html'));
});

app.get('/franchiser/customers', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'customers.html'));
});

app.get('/franchiser/invoices', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'invoices.html'));
});

app.get('/franchisee/login', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'franchisee_login.html'));
});

app.get('/franchisee/items', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'items.html'));
});


app.listen(port, () => {
  console.log(`Server running at http://localhost:${port}`);
});