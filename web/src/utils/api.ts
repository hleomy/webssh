import axios, { AxiosInstance } from 'axios'

const TOKEN_KEY = 'webssh_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

const api: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000
})

api.interceptors.request.use((config) => {
  const token = getToken()
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      clearToken()
      if (!location.pathname.startsWith('/login') && !location.pathname.startsWith('/register')) {
        location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export interface ApiResponse<T = any> {
  code: number
  msg?: string
  data?: T
}

export async function get<T = any>(url: string, params?: any): Promise<T> {
  const r = await api.get<ApiResponse<T>>(url, { params })
  if (r.data.code !== 0) throw new Error(r.data.msg || '请求失败')
  return r.data.data as T
}

export async function post<T = any>(url: string, body?: any, config?: any): Promise<T> {
  const r = await api.post<ApiResponse<T>>(url, body, config)
  if (r.data.code !== 0) throw new Error(r.data.msg || '请求失败')
  return r.data.data as T
}

export async function put<T = any>(url: string, body?: any): Promise<T> {
  const r = await api.put<ApiResponse<T>>(url, body)
  if (r.data.code !== 0) throw new Error(r.data.msg || '请求失败')
  return r.data.data as T
}

export async function del<T = any>(url: string): Promise<T> {
  const r = await api.delete<ApiResponse<T>>(url)
  if (r.data.code !== 0) throw new Error(r.data.msg || '请求失败')
  return r.data.data as T
}

export default api
