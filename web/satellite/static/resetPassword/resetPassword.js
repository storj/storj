/* Copyright (C) 2019 Storj Labs, Inc. */
/* See LICENSE for copying information. */

const passwordType = 'password';
const textType = 'text';
const regularPasswordLabel = 'New Password';
const regularRepeatPasswordLabel = 'Repeat Password';
const errorPasswordLabel = 'Password must contains at least 6 characters';
const errorRepeatPasswordLabel = 'Password doesn\'t match';
const regularColor = '#354049';
const errorColor = '#eb5757';
const passwordInputId = 'passwordInput';
const repeatPasswordId = 'repeatPasswordInput';
const types = {
    PASSWORD: 'PASSWORD',
    REPEAT_PASSWORD: 'REPEAT_PASSWORD',
};

function togglePasswordVisibility() {
    toggleVisibility(passwordInputId);
}

function toggleRepeatPasswordVisibility() {
    toggleVisibility(repeatPasswordId);
}

function toggleVisibility(id) {
    const element = document.getElementById(id);

    if (element) {
        const isPasswordVisible = element.type === textType;

        toggleEyeIcon(id, isPasswordVisible);

        element.type = isPasswordVisible ? passwordType : textType;
    }
}

function toggleEyeIcon(id, isPasswordVisible) {
    const eyeIcon = document.getElementById(`${id}_eyeIcon`);

    if (eyeIcon) {
        eyeIcon.src = isPasswordVisible ?
            '/static/static/images/common/passwordHidden.svg' :
            '/static/static/images/common/passwordShown.svg'
    }
}

function submit() {
    const passwordInput = document.getElementById(passwordInputId);
    if (passwordInput) {
        if (!validatePassword(passwordInput.value)) {
            setPasswordError(true);

            return;
        }
    }

    const repeatPasswordInput = document.getElementById(repeatPasswordId);
    if (repeatPasswordInput) {
        if (passwordInput.value !== repeatPasswordInput.value) {
            setRepeatPasswordError(true);

            return;
        }
    }

    document.resetPasswordForm.submit();
}

function setPasswordError(status) {
    setError(types.PASSWORD, status);
}

function setRepeatPasswordError(status) {
    setError(types.REPEAT_PASSWORD, status);
}

function setError(type, status) {
    let passwordLabel;
    let passwordInput;
    let regularLabel;
    let errorLabel;
    switch (type) {
        case types.PASSWORD:
            passwordLabel = document.getElementById('passwordLabel');
            passwordInput = document.getElementById(passwordInputId);
            regularLabel = regularPasswordLabel;
            errorLabel = errorPasswordLabel;

            break;
        case types.REPEAT_PASSWORD:
            passwordLabel = document.getElementById('repeatPasswordLabel');
            passwordInput = document.getElementById(repeatPasswordId);
            regularLabel = regularRepeatPasswordLabel;
            errorLabel = errorRepeatPasswordLabel;
    }

    if (passwordLabel && passwordInput) {
        changeStyling({
            labelElement: passwordLabel,
            inputElement: passwordInput,
            regularLabel,
            errorLabel,
            status,
        });
    }
}

function changeStyling(configuration) {
    if (configuration.status) {
        configuration.labelElement.innerText = configuration.errorLabel;
        configuration.labelElement.style.color = configuration.inputElement.style.borderColor = errorColor;

        return;
    }

    configuration.labelElement.innerText = configuration.regularLabel;
    configuration.labelElement.style.color = configuration.inputElement.style.borderColor = regularColor;
}

function validatePassword(password) {
    return typeof password !== 'undefined' && password.length >= 6;
}

function resetPasswordError() {
    setError(types.PASSWORD, false);
}

function resetRepeatPasswordError() {
    setError(types.REPEAT_PASSWORD, false);
}
