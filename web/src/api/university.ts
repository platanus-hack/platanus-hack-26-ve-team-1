import { request } from './client'
import { mockGetStudents, mockIssueUniversityCredential } from './mock'
import type { Student } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
// Ambas son supuestos del frontend: pendientes de confirmar con P1 (ver docs/PENDIENTES_P1.md).
export const UNIVERSITY_ENDPOINTS = {
  // GET → lista de estudiantes (solo rol emisor).
  students: '/students',
  // POST { estudianteId } → emite el carnet universitario del estudiante.
  issueCredential: '/credentials/university',
} as const

export function getStudents(): Promise<Student[]> {
  if (useMock) return mockGetStudents()
  return request<Student[]>(UNIVERSITY_ENDPOINTS.students)
}

export function issueUniversityCredential(studentId: string): Promise<void> {
  if (useMock) return mockIssueUniversityCredential(studentId)
  return request<void>(UNIVERSITY_ENDPOINTS.issueCredential, {
    method: 'POST',
    body: { estudianteId: studentId },
  })
}
