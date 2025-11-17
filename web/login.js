// Login form handler
document.getElementById('loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();

    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    const errorDiv = document.getElementById('errorMessage');
    const loginBtn = document.getElementById('loginBtn');

    // Clear previous errors
    errorDiv.style.display = 'none';
    errorDiv.textContent = '';

    // Disable button during submission
    loginBtn.disabled = true;
    loginBtn.textContent = 'Signing in...';

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });

        if (response.ok) {
            // Successful login - redirect to main app
            window.location.href = '/';
        } else {
            // Failed login - show error
            const data = await response.json().catch(() => ({ error: 'Invalid credentials' }));
            errorDiv.textContent = data.error || 'Invalid username or password';
            errorDiv.style.display = 'block';

            // Re-enable button
            loginBtn.disabled = false;
            loginBtn.textContent = 'Sign In';

            // Clear password field
            document.getElementById('password').value = '';
            document.getElementById('password').focus();
        }
    } catch (error) {
        console.error('Login error:', error);
        errorDiv.textContent = 'Login failed. Please check your network connection and try again.';
        errorDiv.style.display = 'block';

        // Re-enable button
        loginBtn.disabled = false;
        loginBtn.textContent = 'Sign In';
    }
});

// Focus username field on load
document.getElementById('username').focus();
