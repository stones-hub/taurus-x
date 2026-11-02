let localReset = () => {}

export function registerLocalReset(reset: () => void) {
  localReset = reset
}

export function clearLocalAuth() {
  localReset()
}

export function createSessionActions<Token>(actions: {
  revoke: () => Promise<unknown>
  rotate: () => Promise<Token>
  clear: () => void
  save: (token: Token) => void
  active: () => boolean
}) {
  let pending: Promise<unknown> = Promise.resolve()
  function serialize<Result>(operation: () => Promise<Result>): Promise<Result> {
    const result = pending.then(operation)
    pending = result.catch(() => {})
    return result
  }
  return {
    logout: () => serialize(async () => {
      await actions.revoke()
      actions.clear()
    }),
    refresh: () => serialize(async () => {
      if (!actions.active()) throw new Error('当前没有可刷新的会话')
      const token = await actions.rotate()
      actions.save(token)
    }),
  }
}
