import { spawn } from 'node:child_process'
import process from 'node:process'

const rootEnv = {
  ...process.env,
  NUXT_UI_TEST_MODE: '1',
  NUXT_APP_BASE_URL: '/',
  NUXT_PUBLIC_API_BASE: '/api/flow/v1',
  NITRO_HOST: '127.0.0.1',
  NITRO_PORT: '4317'
}

const run = (command, args, env = rootEnv) => new Promise((resolve, reject) => {
  const child = spawn(command, args, {
    stdio: 'inherit',
    env,
    shell: false
  })

  child.on('error', reject)
  child.on('exit', (code, signal) => {
    if (code === 0) return resolve(undefined)
    reject(new Error(`${command} ${args.join(' ')} exited with code ${code ?? 'null'} signal ${signal ?? 'none'}`))
  })
})

const main = async () => {
  await run('pnpm', ['build'])

  const server = spawn(process.execPath, ['.output/server/index.mjs'], {
    stdio: 'inherit',
    env: rootEnv,
    shell: false
  })

  const forwardSignal = (signal) => {
    if (!server.killed) server.kill(signal)
  }

  process.on('SIGINT', forwardSignal)
  process.on('SIGTERM', forwardSignal)

  server.on('error', (error) => {
    console.error(error)
    process.exit(1)
  })

  server.on('exit', (code, signal) => {
    if (signal) {
      process.exit(0)
      return
    }
    process.exit(code ?? 0)
  })
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
