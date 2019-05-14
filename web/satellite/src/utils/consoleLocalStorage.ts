
const localStorageConstants = {
    USER_ID: 'userID',
    USER_EMAIL: 'userEmail'
};

export function setUserId(userID: string) {
    localStorage.setItem(localStorageConstants.USER_ID, userID);
}

export function getUserID() {
    return localStorage.getItem(localStorageConstants.USER_ID);
}

export function setUserEmail(userEmail: string) {
    localStorage.setItem(localStorageConstants.USER_EMAIL, userEmail);
}

export function getUserEmail() {
    return localStorage.getItem(localStorageConstants.USER_EMAIL);
}
