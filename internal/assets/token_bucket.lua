-- keys[1]: User ID key (e.g., "rate_limit:user:123")
-- argv[1]: Capacity
-- argv[2]: Refill Rate
-- argv[3]: Requested tokens
-- argv[4]: Current Time (Unix)
-- argv[5]: Abuse - Max Violations (e.g., 5)
-- argv[6]: Abuse - Jail Time in seconds (e.g., 60)

local key = KEYS[1]
local jail_key = key .. ":jail"
local violation_key = key .. ":violations"

local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
local max_violations = tonumber(ARGV[5])
local jail_time = tonumber(ARGV[6])

-- 1. CHECK JAIL (Is the user currently banned?)
if redis.call("EXISTS", jail_key) == 1 then
    return -1 -- Code for "BANNED"
end

-- 2. TOKEN BUCKET LOGIC
local info = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(info[1])
local last_refill = tonumber(info[2])

if tokens == nil then
  tokens = capacity
  last_refill = now
end

local delta = math.max(0, now - last_refill)
local tokens_to_add = delta * refill_rate
tokens = math.min(capacity, tokens + tokens_to_add)

local allowed = 0
if tokens >= requested then
  allowed = 1
  tokens = tokens - requested
  -- Optional: If allowed, we could decrease violation count (Forgiveness), 
  -- but we'll keep it strict for now.
else
  -- 3. ABUSE DETECTION (User was Denied)
  allowed = 0
  
  -- Increment violation counter
  local violations = redis.call("INCR", violation_key)
  -- Reset violation count if they behave for 60 seconds
  redis.call("EXPIRE", violation_key, 60)

  if violations >= max_violations then
      -- BAN THEM: Create the jail key
      redis.call("SETEX", jail_key, jail_time, "banned")
      return -1 -- Code for "JUST BANNED"
  end
end

-- Save new token state
redis.call("HMSET", key, "tokens", tokens, "last_refill", now)
redis.call("EXPIRE", key, 60)

return allowed