document.addEventListener('DOMContentLoaded', () => {
    const form = document.querySelector('form');
    const passwordInput = document.getElementById('password');
    const passwordConfirmInput = document.getElementById('password_confirm');
    const passwordMatchError = document.getElementById('password-match-error');
    const passwordStrengthIndicator = document.getElementById('password-strength');
    const passwordStrengthText = passwordStrengthIndicator.querySelector('.strength-text');
    const submitButton = form.querySelector('.btn-primary');

    function checkPasswordStrength(password) {
        let strength = 0;
        let feedback = "8자 이상 입력하세요";

        if (!password) {
            passwordStrengthIndicator.style.display = 'none';
            return false;
        }

        passwordStrengthIndicator.style.display = 'flex';

        // 1. 길이 체크
        const isLongEnough = password.length >= 8;
        
        // 2. 구성 요소 체크 (대문자 필수 제외)
        const hasAlpha = /[a-zA-Z]/.test(password);
        const hasNum = /[0-9]/.test(password);
        const hasSpecial = /[^A-Za-z0-9]/.test(password);

        if (hasAlpha) strength++;
        if (hasNum) strength++;
        if (hasSpecial) strength++;

        // UI 상태 업데이트
        passwordStrengthIndicator.className = 'password-strength'; // 초기화

        if (!isLongEnough) {
            passwordStrengthIndicator.classList.add('weak');
            passwordStrengthText.textContent = `취약 (8자 이상 필요)`;
            return false;
        }

        if (strength < 2) {
            passwordStrengthIndicator.classList.add('weak');
            passwordStrengthText.textContent = `취약 (문자/숫자/특수문자 조합)`;
            return false;
        } else if (strength === 2) {
            passwordStrengthIndicator.classList.add('medium');
            passwordStrengthText.textContent = `보통`;
            return true;
        } else {
            passwordStrengthIndicator.classList.add('strong');
            passwordStrengthText.textContent = `강력함`;
            return true;
        }
    }

    function validatePasswordsMatch() {
        if (!passwordConfirmInput.value) {
            passwordMatchError.style.display = 'none';
            return false;
        }
        if (passwordInput.value === passwordConfirmInput.value) {
            passwordConfirmInput.classList.remove('error');
            passwordMatchError.style.display = 'none';
            return true;
        } else {
            passwordConfirmInput.classList.add('error');
            passwordMatchError.style.display = 'block';
            return false;
        }
    }

    function updateSubmitButtonState() {
        const isPasswordStrong = checkPasswordStrength(passwordInput.value);
        const passwordsMatch = validatePasswordsMatch();
        const username = document.querySelector('[name="username"]').value.trim();
        const otpToken = document.querySelector('[name="otp_token"]').value.trim();

        submitButton.disabled = !(isPasswordStrong && passwordsMatch && username && otpToken.length === 6);
    }

    passwordInput.addEventListener('input', updateSubmitButtonState);
    passwordConfirmInput.addEventListener('input', updateSubmitButtonState);
    document.querySelector('[name="username"]').addEventListener('input', updateSubmitButtonState);
    document.querySelector('[name="otp_token"]').addEventListener('input', updateSubmitButtonState);

    updateSubmitButtonState();
});
