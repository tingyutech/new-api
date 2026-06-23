为“透传”类型的 channel 增加自动追踪功能：在向上游发送请求的时候，追加 X-NewApi-Request-Id 值为 request id， X-NewApi-User 值为请求用户的用户名， X-NewApi-User-Id 值为请求用户的用户 ID。不需要加设置，直接默认追加就可以了。
