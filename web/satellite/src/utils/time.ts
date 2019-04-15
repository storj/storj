// toUnixTimestamp converts Date to unix timestamp
export function toUnixTimestamp(time :Date) : number {
    return Math.floor(time.getTime() / 1000);
}
