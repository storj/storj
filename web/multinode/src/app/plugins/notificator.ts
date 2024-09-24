import { store } from "@/app/store";
import { NotificationPayload } from "@/app/types/delayedNotification";

export class notify {
  constructor() {}

  public success(payload: NotificationPayload) {
    if (!payload.title) {
      payload.title = "Success";
    }
    store.dispatch("notification/success", payload);
  }

  public error(payload: NotificationPayload) {
    if (!payload.title) {
      payload.title = "Error";
    }
    store.dispatch("notification/error", payload);
  }

  public warning(payload: NotificationPayload) {
    if (!payload.title) {
      payload.title = "Warning";
    }
    store.dispatch("notification/warning", payload);
  }

  public info(payload: NotificationPayload) {
    if (!payload.title) {
      payload.title = "Info";
    }
    store.dispatch("notification/info", payload);
  }
}
