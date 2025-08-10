import React, { useState } from 'react';

function App() {
  const [token, setToken] = useState(null);

  async function register() {
    await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'demo@example.com', password: 'demo' }),
    });
  }

  async function login() {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'demo@example.com', password: 'demo' }),
    });
    const data = await res.json();
    setToken(data.token);
  }

  return (
    <div>
      <h1>FileBox</h1>
      <button onClick={register}>Register</button>
      <button onClick={login}>Login</button>
      {token && <p>Token: {token}</p>}
    </div>
  );
}

export default App;
