thread_id_seed = 0

function setup(thread)
  thread:set("tid", thread_id_seed)
  thread_id_seed = thread_id_seed + 1
end

function init(args)
  local token_file = string.format("/tmp/like_tokens.%02d", tid)

  tokens = {}
  for line in io.lines(token_file) do
    if line ~= "" then
      table.insert(tokens, line)
    end
  end
  if #tokens == 0 then
    error("empty token file: " .. token_file)
  end

  videos = {}
  for line in io.lines("/tmp/video_ids.txt") do
    local id = tonumber(line)
    if id ~= nil then
      table.insert(videos, id)
    end
  end
  if #videos == 0 then
    error("empty /tmp/video_ids.txt")
  end

  wrk.method = "POST"
  wrk.headers["Content-Type"] = "application/json"
  seqno = 0
end

request = function()
  seqno = seqno + 1

  local token_idx = ((seqno - 1) % #tokens) + 1
  local video_round = math.floor((seqno - 1) / #tokens)
  local video_idx = (video_round % #videos) + 1

  wrk.headers["Authorization"] = "Bearer " .. tokens[token_idx]
  local body = string.format('{"video_id":%d}', videos[video_idx])

  return wrk.format("POST", "/like/like", nil, body)
end
