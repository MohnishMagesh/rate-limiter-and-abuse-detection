-- keys[1]: The unique identifier for the user (e.g., "rate_limit:user:123")
-- argv[1]: Max Capacity (e.g., 10 tokens)
-- argv[2]: Refill Rate (e.g., 1 token per second)
-- argv[3]: Requests requested (usually 1)
-- argv[4]: Current Window Time (passed from Go to ensure consistency across servers)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Get current tokens and last_refill_time from Redis hash
local info = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(info[1])
local last_refill = tonumber(info[2])

-- Initialize if missing (first time user is seen)
if tokens == nil then
  tokens = capacity
  last_refill = now
end

-- Calculate how many tokens to add based on time passed
local delta = math.max(0, now - last_refill)
local tokens_to_add = delta * refill_rate

-- Update token count (cannot exceed capacity)
tokens = math.min(capacity, tokens + tokens_to_add)

-- Check if we have enough tokens
local allowed = 0
if tokens >= requested then
  allowed = 1
  tokens = tokens - requested
end

-- Save the new state
redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
-- Set an expiration so inactive users don't clutter RAM (e.g., 60 seconds)
redis.call('EXPIRE', key, 60)

return allowed