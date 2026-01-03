document.addEventListener('DOMContentLoaded', () => {
    const form = document.querySelector('form');
    const submitButton = form.querySelector('.btn-primary');

    const usernameInput = form.querySelector('[name="username"]');
    const passwordInput = form.querySelector('[name="password"]');
    const otpInput = form.querySelector('[name="otp_token"]');

    function updateSubmit() {
        const usernameOk = usernameInput.value.trim().length > 0;
        const passwordOk = passwordInput.value.length > 0;
        const otpOk = otpInput.value.trim().length === 6; // 6자리일 때만
        submitButton.disabled = !(usernameOk && passwordOk && otpOk);
    }

    usernameInput.addEventListener('input', updateSubmit);
    passwordInput.addEventListener('input', updateSubmit);

    otpInput.addEventListener('input', () => {
        // 숫자만 + 최대 6자리
        otpInput.value = otpInput.value.replace(/[^0-9]/g, '').slice(0, 6);
        updateSubmit();
    });

    updateSubmit();
});
