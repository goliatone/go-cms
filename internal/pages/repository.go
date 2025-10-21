package pages

type Repository interface {
	Save(Page) error
}

type NoOpRepository struct{}

func NewNoOpRepository() Repository {
	return NoOpRepository{}
}

func (NoOpRepository) Save(Page) error {
	return nil
}
