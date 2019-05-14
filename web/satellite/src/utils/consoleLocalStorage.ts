
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
