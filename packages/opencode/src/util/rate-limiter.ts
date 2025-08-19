import { Log } from "./log"

export namespace RateLimiter {
  const log = Log.create({ service: "rate-limiter" })

  // Rate limiting: Map of providerID -> timestamps array (sliding window)
  const rateLimitWindows = new Map<string, number[]>()

  // Track if any provider is currently rate limited
  let isRateLimited = false
  let rateLimitedProvider = ""
  let rateLimitWaitUntil = 0 // Timestamp when rate limit ends

  async function sleep(ms: number) {
    return new Promise((resolve) => setTimeout(resolve, ms))
  }

  async function waitForRateLimit(key: string, rpmLimit: number) {
    const now = Date.now()
    const windowMs = 60000 // 1 minute in milliseconds
    const windowStart = now - windowMs

    // Get or create window for this key (providerID/modelID)
    let window = rateLimitWindows.get(key)
    if (!window) {
      window = []
      rateLimitWindows.set(key, window)
    }

    // Remove old requests outside the sliding window
    while (window.length > 0 && window[0] < windowStart) {
      window.shift()
    }

    // Check if we're at the limit
    if (window.length > rpmLimit) {
      // Calculate how long to wait until the oldest request expires
      const oldestRequest = window[0]
      const waitTime = oldestRequest + windowMs - now

      if (waitTime > 0) {
        // Set global rate limit status
        isRateLimited = true
        rateLimitedProvider = key
        rateLimitWaitUntil = now + waitTime

        log.info("rate limit hit, sleeping", {
          key,
          rpmLimit,
          currentRequests: window.length,
          waitTimeMs: waitTime,
        })

        await sleep(waitTime)

        // Clear rate limit status
        isRateLimited = false
        rateLimitedProvider = ""
        rateLimitWaitUntil = 0

        // Recursively check again in case multiple requests are waiting
        return waitForRateLimit(key, rpmLimit)
      }
    }

    // Record this request
    window.push(now)
  }

  export async function checkRateLimit(providerID: string, modelID: string, config?: any) {
    try {
      if (!config) return
      const providerConfig = config.provider?.[providerID]
      let rpmLimit: number | undefined = undefined
      // Check for model-level rpm in the limit object
      if (providerConfig?.models && modelID && providerConfig.models[modelID]?.limit?.rpm) {
        rpmLimit = providerConfig.models[modelID].limit.rpm
      }
      if (rpmLimit) {
        await waitForRateLimit(`${providerID}/${modelID}`, rpmLimit)
      }
    } catch (error) {
      log.warn("failed to apply rate limiting", { error, providerID, modelID })
    }
  }

  export function isCurrentlyRateLimited(): boolean {
    return isRateLimited
  }

  export function getRateLimitedProvider(): string {
    return rateLimitedProvider
  }

  export function getCurrentWaitTime(): number {
    if (!isRateLimited || !rateLimitedProvider || rateLimitWaitUntil === 0) return 0

    const now = Date.now()
    const remainingMs = rateLimitWaitUntil - now

    if (remainingMs <= 0) {
      // Rate limit has expired, clear it
      isRateLimited = false
      rateLimitedProvider = ""
      rateLimitWaitUntil = 0
      return 0
    }

    return Math.ceil(remainingMs / 1000) // Convert to seconds and round up
  }
}
