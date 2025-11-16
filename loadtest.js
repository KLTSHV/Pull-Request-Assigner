import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 20,          // количество виртуальных пользователей
  duration: '30s',  // длительность теста
};

// В рамках задачи нас интересуют:
// - /users/getReview — выборка PR по ревьюверу
// - /pullRequest/merge — идемпотентный merge
// - /stats — простая агрегированная статистика
export default function () {
  // 1. Получить PRы ревьювера u2
  const resReviews = http.get('http://localhost:8080/users/getReview?user_id=u2');
  check(resReviews, {
    'getReview status 200': (r) => r.status === 200,
  });

  // 2. Идемпотентный merge одного PR
  const resMerge = http.post(
    'http://localhost:8080/pullRequest/merge',
    JSON.stringify({ pull_request_id: 'pr-1' }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(resMerge, {
    'merge status 200': (r) => r.status === 200,
  });

  // 3. Статистика
  const resStats = http.get('http://localhost:8080/stats');
  check(resStats, {
    'stats status 200': (r) => r.status === 200,
  });

  // чтобы не долбить слишком часто
  sleep(0.1);
}
