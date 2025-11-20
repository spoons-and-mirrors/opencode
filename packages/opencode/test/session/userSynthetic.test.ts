import { describe, expect, test } from "bun:test"
import { MessageV2 } from "../../src/session/message-v2"

describe("userSynthetic messages", () => {
  test("should filter userSynthetic text parts from toModelMessage", () => {
    const messages: { info: MessageV2.Info; parts: MessageV2.Part[] }[] = [
      {
        info: {
          id: "msg1",
          role: "user",
          time: { start: Date.now() },
        },
        parts: [
          {
            id: "part1",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "This should be sent to LLM",
          } as MessageV2.TextPart,
          {
            id: "part2",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "This is userSynthetic and should NOT be sent to LLM",
            userSynthetic: true,
          } as MessageV2.TextPart,
        ],
      },
    ]

    const modelMessages = MessageV2.toModelMessage(messages)

    // Should have one user message
    expect(modelMessages.length).toBe(1)
    expect(modelMessages[0].role).toBe("user")

    // Should only contain the non-userSynthetic text
    const content = modelMessages[0].content
    expect(typeof content).toBe("string")
    expect(content).toBe("This should be sent to LLM")
    expect(content).not.toContain("userSynthetic")
  })

  test("should include normal synthetic parts in toModelMessage", () => {
    const messages: { info: MessageV2.Info; parts: MessageV2.Part[] }[] = [
      {
        info: {
          id: "msg1",
          role: "user",
          time: { start: Date.now() },
        },
        parts: [
          {
            id: "part1",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "Regular text",
          } as MessageV2.TextPart,
          {
            id: "part2",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "Synthetic text for LLM only",
            synthetic: true,
          } as MessageV2.TextPart,
        ],
      },
    ]

    const modelMessages = MessageV2.toModelMessage(messages)

    // Should include both texts since synthetic parts are sent to LLM
    const content = modelMessages[0].content as string
    expect(content).toContain("Regular text")
    expect(content).toContain("Synthetic text for LLM only")
  })

  test("should handle messages with both synthetic and userSynthetic parts", () => {
    const messages: { info: MessageV2.Info; parts: MessageV2.Part[] }[] = [
      {
        info: {
          id: "msg1",
          role: "user",
          time: { start: Date.now() },
        },
        parts: [
          {
            id: "part1",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "Regular text",
          } as MessageV2.TextPart,
          {
            id: "part2",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "Synthetic (for LLM)",
            synthetic: true,
          } as MessageV2.TextPart,
          {
            id: "part3",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "UserSynthetic (for UI only)",
            userSynthetic: true,
          } as MessageV2.TextPart,
        ],
      },
    ]

    const modelMessages = MessageV2.toModelMessage(messages)

    const content = modelMessages[0].content as string
    // Should include regular and synthetic but not userSynthetic
    expect(content).toContain("Regular text")
    expect(content).toContain("Synthetic (for LLM)")
    expect(content).not.toContain("UserSynthetic")
  })

  test("should handle messages with only userSynthetic parts", () => {
    const messages: { info: MessageV2.Info; parts: MessageV2.Part[] }[] = [
      {
        info: {
          id: "msg1",
          role: "user",
          time: { start: Date.now() },
        },
        parts: [
          {
            id: "part1",
            sessionID: "session1",
            messageID: "msg1",
            type: "text",
            text: "Only userSynthetic text",
            userSynthetic: true,
          } as MessageV2.TextPart,
        ],
      },
    ]

    const modelMessages = MessageV2.toModelMessage(messages)

    // Message should be filtered out or have empty content
    // since all parts are userSynthetic
    expect(modelMessages.length).toBe(0)
  })
})
