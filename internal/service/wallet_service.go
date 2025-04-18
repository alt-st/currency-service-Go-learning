// --- internal/service/wallet_service.go ---
// (!!!) Новый файл
package service

import (
	"context"
	"database/sql"
	"errors" // Для стандартных ошибок
	"fmt"
	"log"
	"regexp" // Для валидации номера кошелька

	"currency-service/internal/models"
	"currency-service/internal/repository"
)

// Определим кастомные ошибки для лучшей обработки в хендлере
var (
	ErrWalletNotFound      = errors.New("кошелек не найден")
	ErrInsufficientFunds   = errors.New("недостаточно средств на счете")
	ErrInvalidWalletNumber = errors.New("некорректный формат номера кошелька (требуется 7 цифр)")
	ErrNegativeDeposit     = errors.New("сумма для создания кошелька должна быть положительной")
	ErrWithdrawNonExistent = errors.New("нельзя списать средства с несуществующего кошелька")
	ErrRateNotAvailable    = errors.New("не удалось получить актуальный курс валют")
)

// Регулярное выражение для проверки номера кошелька (ровно 7 цифр)
var walletNumberRegex = regexp.MustCompile(`^\d{7}$`)

type walletService struct {
	walletRepo repository.WalletRepository
	rateRepo   repository.RateRepository // Добавляем зависимость для получения курса
	db         *sql.DB                   // Для управления транзакциями
}

// NewWalletService создает новый экземпляр сервиса кошельков.
func NewWalletService(walletRepo repository.WalletRepository, rateRepo repository.RateRepository, db *sql.DB) WalletService {
	return &walletService{
		walletRepo: walletRepo,
		rateRepo:   rateRepo,
		db:         db,
	}
}

// Helper function to execute database operations within a transaction
func (s *walletService) executeTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil) // Начинаем транзакцию
	if err != nil {
		log.Printf("Ошибка начала транзакции: %v", err)
		return fmt.Errorf("внутренняя ошибка сервера (tx begin): %w", err)
	}

	err = fn(tx) // Выполняем переданную функцию внутри транзакции

	if err != nil {
		// Если функция вернула ошибку, откатываем транзакцию
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Ошибка отката транзакции после ошибки %v: %v", err, rbErr)
			return fmt.Errorf("внутренняя ошибка сервера (tx rollback after error): %w", err) // Возвращаем исходную ошибку
		}
		log.Printf("Транзакция отменена из-за ошибки: %v", err)
		return err // Возвращаем исходную ошибку (например, ErrInsufficientFunds)
	}

	// Если функция выполнилась успешно, коммитим транзакцию
	if err := tx.Commit(); err != nil {
		log.Printf("Ошибка коммита транзакции: %v", err)
		return fmt.Errorf("внутренняя ошибка сервера (tx commit): %w", err)
	}

	log.Println("Транзакция успешно завершена")
	return nil
}

// UpdateBalance создает или обновляет баланс кошелька.
func (s *walletService) UpdateBalance(ctx context.Context, req models.UpdateBalanceRequest) (models.UpdateBalanceResponse, error) {
	// 1. Валидация номера кошелька
	if !walletNumberRegex.MatchString(req.WalletNumber) {
		return models.UpdateBalanceResponse{}, ErrInvalidWalletNumber
	}

	var finalBalance float64
	var message string

	// 2. Выполняем операцию в транзакции
	err := s.executeTx(ctx, func(tx *sql.Tx) error {
		// Пытаемся получить кошелек с блокировкой
		wallet, err := s.walletRepo.GetWalletByNumberForUpdate(ctx, tx, req.WalletNumber)

		if err != nil {
			// Если кошелек НЕ найден
			if errors.Is(err, sql.ErrNoRows) {
				// Создать можно только с положительной суммой
				if req.Amount <= 0 {
					return ErrWithdrawNonExistent // Нельзя списать или создать с нулевым/отрицательным балансом
				}
				// Создаем новый кошелек
				newWallet := models.Wallet{
					Number:  req.WalletNumber,
					Balance: req.Amount,
				}
				if createErr := s.walletRepo.CreateWallet(ctx, tx, newWallet); createErr != nil {
					// Ошибка создания может быть из-за гонки (другой запрос успел создать) - проверяем
					if createErr.Error() == fmt.Sprintf("кошелек с номером %s уже существует", req.WalletNumber) {
						// Повторяем попытку получить кошелек, чтобы обработать как обновление
						// Это редкий случай, но возможный при высокой конкуренции
						log.Printf("Гонка при создании кошелька %s, повторная попытка...", req.WalletNumber)
						// Можно было бы сделать goto или рефакторинг, но пока просто вернем ошибку для повтора операции клиентом
						// или реализовать более сложный механизм повтора внутри сервиса
						return fmt.Errorf("конфликт при создании кошелька, попробуйте еще раз")
					}
					return fmt.Errorf("не удалось создать кошелек: %w", createErr)
				}
				finalBalance = newWallet.Balance
				message = "Кошелек успешно создан"
				return nil // Успешное создание
			}
			// Другая ошибка при получении кошелька
			return fmt.Errorf("ошибка получения кошелька: %w", err)
		}

		// Если кошелек НАЙДЕН - обновляем баланс
		newBalance := wallet.Balance + req.Amount
		if newBalance < 0 {
			// Недостаточно средств для списания
			finalBalance = wallet.Balance // Баланс не меняется
			message = "Недостаточно средств для списания"
			return ErrInsufficientFunds // Возвращаем ошибку, транзакция будет отменена
		}

		// Обновляем баланс в БД
		if updateErr := s.walletRepo.UpdateWalletBalance(ctx, tx, req.WalletNumber, newBalance); updateErr != nil {
			return fmt.Errorf("не удалось обновить баланс: %w", updateErr)
		}
		finalBalance = newBalance
		if req.Amount >= 0 {
			message = "Баланс успешно пополнен"
		} else {
			message = "Списание успешно выполнено"
		}
		return nil // Успешное обновление
	})

	// 3. Обработка результата транзакции
	if err != nil {
		// Если была ошибка (например, InsufficientFunds), возвращаем ее
		// и пустой ответ с сообщением об ошибке
		// Определяем, какое сообщение вернуть клиенту
		userMessage := "Не удалось обновить баланс"
		if errors.Is(err, ErrInsufficientFunds) {
			userMessage = ErrInsufficientFunds.Error()
		} else if errors.Is(err, ErrWithdrawNonExistent) {
			userMessage = ErrWithdrawNonExistent.Error()
		}
		// Не возвращаем сам err, если это внутренняя ошибка, а возвращаем userMessage
		// Но если это 'бизнес-ошибка', то можно ее и вернуть
		log.Printf("Ошибка в UpdateBalance после транзакции: %v", err)
		// Возвращаем структуру ответа с сообщением об ошибке
		return models.UpdateBalanceResponse{
			WalletNumber: req.WalletNumber,
			NewBalance:   finalBalance, // Показываем баланс на момент ошибки (если он был прочитан)
			Message:      userMessage,
		}, err // Возвращаем саму ошибку для определения статуса HTTP в хендлере
	}

	// Если транзакция прошла успешно
	return models.UpdateBalanceResponse{
		WalletNumber: req.WalletNumber,
		NewBalance:   finalBalance,
		Message:      message,
	}, nil
}

// ListWallets возвращает список всех кошельков.
func (s *walletService) ListWallets(ctx context.Context) (models.ListWalletsResponse, error) {
	wallets, err := s.walletRepo.GetAllWallets(ctx, s.db) // Используем *sql.DB напрямую, транзакция не нужна
	if err != nil {
		log.Printf("Ошибка получения списка кошельков в сервисе: %v", err)
		return models.ListWalletsResponse{}, fmt.Errorf("не удалось получить список кошельков: %w", err)
	}
	return models.ListWalletsResponse{Wallets: wallets}, nil
}

// ConvertAndDeduct выполняет конвертацию и списание средств.
func (s *walletService) ConvertAndDeduct(ctx context.Context, req models.ConvertRequest) (models.ConvertResponse, error) {
	// 1. Валидация
	if !walletNumberRegex.MatchString(req.SourceWalletNumber) {
		return models.ConvertResponse{Message: ErrInvalidWalletNumber.Error()}, ErrInvalidWalletNumber
	}
	if req.AmountToConvert <= 0 {
		err := errors.New("сумма для конвертации должна быть положительной")
		return models.ConvertResponse{Message: err.Error()}, err
	}

	var finalResponse models.ConvertResponse
	finalResponse.SourceWalletNumber = req.SourceWalletNumber // Заполняем сразу

	// 2. Получаем самый свежий курс (вне транзакции, т.к. курс может меняться)
	// В реальном приложении курс может кешироваться или браться на момент начала операции
	latestRate, err := s.rateRepo.GetLatestRate(ctx, s.db)
	if err != nil {
		log.Printf("Ошибка получения курса для конвертации: %v", err)
		finalResponse.Message = ErrRateNotAvailable.Error()
		return finalResponse, ErrRateNotAvailable
	}
	finalResponse.RateUsed = latestRate.Value // Сохраняем курс для ответа

	// Сумма к списанию равна сумме для конвертации (по условию)
	//amountToDeduct := req.AmountToConvert
	//convertedAmount := req.AmountToConvert * latestRate.Value // Пример расчета конвертированной суммы
	//finalResponse.ConvertedAmount = convertedAmount
	// Сумма к списанию это исходная сумму умноженная на коэффициент конвертации
	amountToDeduct := req.AmountToConvert * latestRate.Value
	finalResponse.ConvertedAmount = amountToDeduct

	// 3. Выполняем проверку и списание в транзакции
	err = s.executeTx(ctx, func(tx *sql.Tx) error {
		// Получаем кошелек с блокировкой
		wallet, err := s.walletRepo.GetWalletByNumberForUpdate(ctx, tx, req.SourceWalletNumber)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrWalletNotFound
			}
			return fmt.Errorf("ошибка получения кошелька для конвертации: %w", err)
		}

		// Проверяем баланс
		if wallet.Balance < amountToDeduct {
			finalResponse.RemainingBalance = wallet.Balance // Показываем текущий баланс
			return ErrInsufficientFunds
		}

		// Списываем средства
		newBalance := wallet.Balance - amountToDeduct
		if updateErr := s.walletRepo.UpdateWalletBalance(ctx, tx, req.SourceWalletNumber, newBalance); updateErr != nil {
			return fmt.Errorf("не удалось списать средства для конвертации: %w", updateErr)
		}

		// Заполняем оставшиеся поля ответа при успехе транзакции
		finalResponse.RemainingBalance = newBalance
		finalResponse.Message = "Конвертация и списание прошли успешно"
		return nil // Успех транзакции
	})

	// 4. Обработка результата транзакции
	if err != nil {
		log.Printf("Ошибка в ConvertAndDeduct после транзакции: %v", err)
		// Заполняем сообщение об ошибке в ответе
		if errors.Is(err, ErrWalletNotFound) {
			finalResponse.Message = ErrWalletNotFound.Error()
		} else if errors.Is(err, ErrInsufficientFunds) {
			finalResponse.Message = ErrInsufficientFunds.Error()
		} else {
			finalResponse.Message = "Ошибка при выполнении конвертации"
		}
		// Возвращаем структуру ответа с сообщением и саму ошибку
		return finalResponse, err
	}

	// Возвращаем успешный ответ
	return finalResponse, nil
}
